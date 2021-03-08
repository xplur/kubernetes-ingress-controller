//+build integration_tests

package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kong/kubernetes-testing-framework/pkg/generators/k8s"
	ktfkind "github.com/kong/kubernetes-testing-framework/pkg/kind"
	ktfmetal "github.com/kong/kubernetes-testing-framework/pkg/metallb"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/kong/kubernetes-ingress-controller/railgun/controllers"
)

var (
	// ClusterName indicates the name of the Kind test cluster setup for this test suite.
	ClusterName = uuid.New().String()

	// kc is a kubernetes clientset for the default Kind cluster created for this test suite.
	kc *kubernetes.Clientset

	// ProxyReadyTimeout is the maximum amount of time the tests will wait for the Kong proxy
	// to become available in the cluster before considering the cluster a failure and panicing. FIXME
	ProxyReadyTimeout = time.Minute * 10

	// ProxyReadyChannel is the channel that indicates when the Kong proxy is ready to use.
	// NOTE: if the proxy doesn't become ready within the timeout, the tests will panic. FIXME
	ProxyReadyChannel = make(chan *url.URL)
)

func TestMain(m *testing.M) {
	// TODO - at some point when it's more mature, the majority of logic here should most likely end up
	// being a runbook over in github.com/kong/kubernetes-testing-framework/pkg/runbook as the environment
	// we build here is fairly generic and a good starting point for a variety of tests involing KIC.

	// setup a kind cluster for testing, this cluster will have the latest stable version of
	// the Kong proxy already running and available.
	err := ktfkind.CreateKindClusterWithProxy(ClusterName)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(10) // TODO - if we move this into a KTF runbook, we should document the exit codes explicitly
	}

	// cleanup the kind cluster when we're done, unless flagged otherwise
	defer func() {
		if v := os.Getenv("KIND_KEEP_CLUSTER"); v == "" { // you can optionally flag the tests to retain the test cluster for inspection.
			if err := ktfkind.DeleteKindCluster(ClusterName); err != nil {
				fmt.Fprintf(os.Stderr, err.Error())
				os.Exit(11)
			}
		}
	}()

	// setup Metallb for the cluster for LoadBalancer addresses for Kong
	if err := ktfmetal.DeployMetallbForKindCluster(ClusterName, ktfkind.DefaultKindDockerNetwork); err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(12)
	}

	// retrieve the *kubernetes.Clientset for the cluster
	kc, err = ktfkind.ClientForKindCluster(ClusterName)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(13)
	}

	// get the kong proxy deployment from the cluster
	ctx, cancel := context.WithCancel(context.Background())
	proxyDeployment, err := kc.AppsV1().Deployments(controllers.DefaultNamespace).Get(ctx, "ingress-controller-kong", metav1.GetOptions{}) // FIXME - race condition here with image ingress deployment // FIXME - race condition here with image ingress deployment
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(15)
	}
	defer cancel()

	// inform the tests when the proxy is ready for use and accessible by producing the working URL
	// TODO - need to handle context in here too.
	proxyLoadBalancerService := k8s.NewServiceForDeployment(proxyDeployment, corev1.ServiceTypeLoadBalancer)
	startProxyInformer(ctx, kc, proxyLoadBalancerService)

	// expose the kong proxy via LoadBalancer service using MetalLB
	proxyLoadBalancerService, err = kc.CoreV1().Services(controllers.DefaultNamespace).Create(ctx, proxyLoadBalancerService, metav1.CreateOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(16)
	}

	// deploy the Kong Kubernetes Ingress Controller (KIC) to the cluster
	// TODO - need to fix the context handling here
	cancel2, err := deployControllers(kc, os.Getenv("KONG_CONTROLLER_TEST_IMAGE"), controllers.DefaultNamespace)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(17)
	}
	defer cancel2()

	// run the tests
	code := m.Run()
	os.Exit(code)
}

// TODO - once this has matured a little we should move this to the testing framework
// as waiting for the proxy service to be accessible will be commonly needed functionality
func startProxyInformer(ctx context.Context, kc *kubernetes.Clientset, watchService *corev1.Service) {
	factory := kubeinformers.NewSharedInformerFactory(kc, ProxyReadyTimeout)
	informer := factory.Core().V1().Services().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObject, newObject interface{}) {
			// TODO - this is messy we need to clean this up and move it into KTF
			svc, ok := newObject.(*corev1.Service)
			if !ok {
				panic(fmt.Errorf("type of %s found", reflect.TypeOf(newObject))) // FIXME
			}

			if svc.Name == watchService.Name {
				ing := svc.Status.LoadBalancer.Ingress
				if len(ing) > 0 && ing[0].IP != "" {
					// FIXME - need error handling and logging output so this isn't hard to debug later if something breaks it
					for _, port := range svc.Spec.Ports {
						if port.Name == "proxy" {
							u, err := url.Parse(fmt.Sprintf("http://%s:%d", ing[0].IP, port.Port))
							if err != nil {
								panic(err) // FIXME
							}
							ProxyReadyChannel <- u
							close(ProxyReadyChannel)
						}
					}
				}
			}
		},
	})
	go informer.Run(ctx.Done())
}

// FIXME: this is a total hack for now
func deployControllers(kc *kubernetes.Clientset, containerImage, namespace string) (context.CancelFunc, error) {
	// ensure the controller namespace is created
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	if _, err := kc.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{}); err != nil {
		if !errors.IsAlreadyExists(err) {
			return nil, err
		}
	}

	// FIXME: temp logging file
	tmpfile, err := ioutil.TempFile(os.TempDir(), "kong-integration-tests-")
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(os.Stdout, "INFO: tempfile for controller logs: %s\n", tmpfile.Name())

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		stderr := new(bytes.Buffer)
		cmd := exec.CommandContext(ctx, "go", "run", "../../main.go", "--kong-url", fmt.Sprintf("http://%s:8001", proxyURL()))
		cmd.Stdout = tmpfile
		cmd.Stderr = io.MultiWriter(stderr, tmpfile)

		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
		}
	}()

	return cancel, nil
}

var prx *url.URL
var lock = sync.Mutex{}

func proxyURL() *url.URL {
	lock.Lock()
	defer lock.Unlock()

	if prx == nil {
		prx = <-ProxyReadyChannel
	}

	return prx
}
