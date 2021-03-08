//+build integration_tests

package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/kong/kubernetes-testing-framework/pkg/generators/k8s"
	"github.com/kong/kubernetes-testing-framework/pkg/runbooks"
	"github.com/stretchr/testify/assert"
)

func TestIngress(t *testing.T) {
	assert.NoError(t, runbooks.DeployIngressForContainer(kc, "kong", "/nginx", k8s.NewContainer("nginx", "nginx", 80)))
	time.Sleep(time.Second * 10) // FIXME
	resp, err := http.Get(fmt.Sprintf("%s/nginx", proxyURL().String()))
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.Status)
}
