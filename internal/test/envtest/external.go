package envtest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
)

func ExternalCRDDirectoryPaths() []string {
	return []string{
		// CAPI v1.14 moved core CRDs from config/crd/bases to core/config/crd/bases.
		filepath.Join(
			getModulePath("", "sigs.k8s.io/cluster-api"),
			"core",
			"config",
			"crd",
			"bases",
		),
		filepath.Join(
			getModulePath("", "sigs.k8s.io/cluster-api"),
			"controlplane",
			"kubeadm",
			"config",
			"crd",
			"bases",
		),
		filepath.Join(
			getModulePath("", "sigs.k8s.io/cluster-api"),
			"bootstrap",
			"kubeadm",
			"config",
			"crd",
			"bases",
		),
	}
}

func ThirdPartyCRDDirectoryPath(thirdPartySubDir, moduleName string, subPath ...string) string {
	_, filename, _, _ := goruntime.Caller(0)
	repoRoot, err := filepath.Abs(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	if err != nil {
		panic(fmt.Sprintf("failed to resolve repo root: %v", err))
	}
	moduleDir := filepath.Join(repoRoot, "hack", "third-party", thirdPartySubDir)
	elems := make([]string, 0, 1+len(subPath))
	elems = append(elems, getModulePath(moduleDir, moduleName))
	elems = append(elems, subPath...)
	return filepath.Join(elems...)
}

func getModulePath(moduleDir, moduleName string) string {
	dir := runGoListModuleDir(moduleDir, moduleName)
	if dir != "" {
		return dir
	}

	dlCmd := exec.Command("go", "mod", "download", moduleName)
	dlCmd.Dir = moduleDir
	dlCmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := dlCmd.CombinedOutput(); err != nil {
		panic(
			fmt.Sprintf("cmd.Dir=%q, cmd.Args=%q, err=%q, output=%q",
				dlCmd.Dir,
				dlCmd.Args,
				err,
				out),
		)
	}

	return runGoListModuleDir(moduleDir, moduleName)
}

func runGoListModuleDir(moduleDir, moduleName string) string {
	cmd := exec.Command("go", "list", "-m", "-f", "{{ .Dir }}", moduleName)
	cmd.Dir = moduleDir
	cmd.Env = append(os.Environ(), "GOWORK=off")
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
