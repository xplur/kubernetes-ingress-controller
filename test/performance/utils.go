package performance

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

func DeployManifest(yml string, ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", yml)
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stdout, stdout.String())
		return err
	}
	fmt.Printf("successfully deploy manifest " + yml)
	return nil
}
