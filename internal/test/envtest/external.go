package envtest

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func ExternalCRDDirectoryPaths() []string {
	return []string{
		filepath.Join(
			getModulePath("sigs.k8s.io/cluster-api"),
			"config",
			"crd",
			"bases",
		),
		filepath.Join(
			getModulePath("sigs.k8s.io/cluster-api"),
			"controlplane",
			"kubeadm",
			"config",
			"crd",
			"bases",
		),
		filepath.Join(
			getModulePath("sigs.k8s.io/cluster-api"),
			"bootstrap",
			"kubeadm",
			"config",
			"crd",
			"bases",
		),
	}
}

func getModulePath(moduleName string) string {
	cmd := exec.Command("go", "list", "-m", "-f", "{{ .Dir }}", moduleName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// We include the combined output because the error is usually
		// an exit code, which does not explain why the command failed.
		panic(
			fmt.Sprintf("cmd.Dir=%q, cmd.Env=%q, cmd.Args=%q, err=%q, output=%q",
				cmd.Dir,
				cmd.Env,
				cmd.Args,
				err,
				out),
		)
	}
	return strings.TrimSpace(string(out))
}
