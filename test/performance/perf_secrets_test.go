//+build performance_tests

package performance

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	secretKeyLen  = 128
	secretsNumber = 2
)

func TestLoadingSecrets(t *testing.T) {
	t.Skip()
	t.Log("setting up the TestIngressPerf")
	ctx := context.Background()
	cluster := env.Cluster()
	cnt := 1
	for cnt <= secretsNumber {
		namespace := fmt.Sprintf("secrets-%d", cnt)
		err := CreateNamespace(ctx, namespace, t)
		assert.NoError(t, err)

		deployK8SSecrets(cluster, namespace, ctx, t)
		cnt += 1
	}
	t.Logf("loaded %d secrets into the cluster.", secretsNumber)
}

func deployK8SSecrets(cluster clusters.Cluster, namespace string, ctx context.Context, t *testing.T) error {
	secretKey := make([]byte, secretKeyLen)
	if _, err := rand.Read(secretKey); err != nil {
		return err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "closepr",
			Namespace: namespace,
		},
		StringData: map[string]string{
			"secretkey": base64.StdEncoding.EncodeToString(secretKey),
		},
	}
	if _, err := cluster.Client().CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{}); err != nil {
		t.Logf("failed creating secrets within namespace %s err %v", namespace, err)
		if !errors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}
