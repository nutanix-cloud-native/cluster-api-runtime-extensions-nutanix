package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	syncHelmValues = "sync-helm-values"
)

var log = ctrl.LoggerFrom(context.Background())

func main() {
	var (
		kustomizeDir    string
		helmTemplateDir string
	)
	args := os.Args
	flagSet := flag.NewFlagSet(
		syncHelmValues,
		flag.ExitOnError,
	)
	flagSet.StringVar(
		&kustomizeDir,
		"kustomize-directory",
		"",
		"Kustomize base directory for all addons",
	)
	flagSet.StringVar(
		&helmTemplateDir,
		"helm-template-directory",
		"",
		"Directory of all the helm templates.",
	)
	err := flagSet.Parse(args[1:])
	if err != nil {
		log.Error(err, "failed to parse args")
	}
	kustomizeDir, err = EnsureFullPath(kustomizeDir)
	if err != nil {
		log.Error(err, "failed to ensure full path for argument")
	}
	helmTemplateDir, err = EnsureFullPath(helmTemplateDir)
	if err != nil {
		log.Error(err, "failed to ensure full path for argument")
	}
	err = Sync(kustomizeDir, helmTemplateDir, func(fileName string) bool {
		if strings.Contains(fileName, "metallb") {
			return true
		}
		if strings.Contains(fileName, "kustomization.yaml.tmpl") {
			return true
		}
		return false
	})
	if err != nil {
		log.Error(err, "failed to sync directories")
	}
}

func EnsureFullPath(filename string) (string, error) {
	if path.IsAbs(filename) {
		return filename, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get wd: %w", err)
	}
	fullPath := path.Join(wd, filename)
	fullPath = path.Clean(fullPath)
	_, err = os.Stat(fullPath)
	if err != nil {
		return "", err
	}
	return fullPath, nil
}

func Sync(sourceDirectory, destDirectory string, shouldSkip func(string) bool) error {
	sourceFS := os.DirFS(sourceDirectory)
	err := fs.WalkDir(sourceFS, ".", func(filepath string, d fs.DirEntry, err error) error {
		if filepath == "." {
			return nil
		}
		if err != nil {
			return err
		}
		if shouldSkip(filepath) {
			return fs.SkipDir
		}
		f := path.Join(destDirectory, filepath)
		_, err = os.Stat(f)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if d.IsDir() {
			return nil // skip it
		}
		_, err = os.Create(f)
		if err != nil {
			fmt.Println("err creating:", err)
			return err
		}
		return nil
	})
	return err
}
