package envtest

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func ExternalCRDDirectoryPaths() []string {
	capiRoot := getModulePath("sigs.k8s.io/cluster-api")
	// CAPI v1.14+ splits CRDs across core / bootstrap / controlplane packages.
	return []string{
		filepath.Join(capiRoot, "core", "config", "crd", "bases"),
		filepath.Join(capiRoot, "controlplane", "kubeadm", "config", "crd", "bases"),
		filepath.Join(capiRoot, "bootstrap", "kubeadm", "config", "crd", "bases"),
	}
}

func getModulePath(moduleName string) string {
	dir := moduleDirFromGoList(moduleName)
	if dir != "" {
		return dir
	}
	// go list can return an empty Dir before the module is in the module cache.
	download := exec.Command("go", "mod", "download", moduleName)
	if out, err := download.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("go mod download %q failed: %v\n%s", moduleName, err, out))
	}
	dir = moduleDirFromGoList(moduleName)
	if dir == "" {
		panic(fmt.Sprintf("module %q has empty Dir after download", moduleName))
	}
	return dir
}

func moduleDirFromGoList(moduleName string) string {
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
