package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"text/template"
	"text/template/parse"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"
)

const (
	syncHelmValues          = "sync-helm-values"
	helmValuesFileName      = "helm-values.yaml"
	helmValuesConfigMapName = "helm-addon-installation.yaml"
)

var log = ctrl.LoggerFrom(context.TODO())

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
	err = SyncHelmValues(kustomizeDir, helmTemplateDir)
	if err != nil {
		fmt.Println("failed to sync err:", err.Error())
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

func SyncHelmValues(sourceDirectory, destDirectory string) error {
	sourceFS := os.DirFS(sourceDirectory)
	err := fs.WalkDir(sourceFS, ".", func(filepath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !strings.Contains(filepath, helmValuesFileName) || strings.Contains(filepath, "tigera") {
			return nil
		}
		sourceFile, err := os.Open(path.Join(sourceDirectory, filepath))
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer sourceFile.Close()
		sourceBytes, err := io.ReadAll(sourceFile)
		sourceString := string(sourceBytes)
		if err != nil {
			return fmt.Errorf("failed to read contents %w", err)
		}
		destPath := getHelmConfigMapFileName(path.Join(destDirectory, filepath))
		destFile, err := os.Open(destPath)
		if err != nil {
			return fmt.Errorf("failed to open %s with error %w", destPath, err)
		}
		defer destFile.Close()
		destFileBytes, err := io.ReadAll(destFile)
		if err != nil {
			return fmt.Errorf("failed to read all bytes of %s got err %w", destPath, err)
		}
		name, templateText, ifPipeline, err := extractTemplateText(string(destFileBytes))
		if err != nil {
			return fmt.Errorf("failed to parse template %w", err)
		}
		cm := corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "ConfigMap",
			},
			Data: make(map[string]string),
		}
		err = yaml.Unmarshal([]byte(templateText), &cm)
		if err != nil {
			return fmt.Errorf("failed to decode into configmap %w", err)
		}
		sourceString = strings.ReplaceAll(
			sourceString,
			"tmpl-clustername-tmpl",
			"\"{{ `{{ .Cluster.Name }}` }}\"",
		)
		sourceString = strings.ReplaceAll(
			sourceString,
			"tmpl-clusternamespace-tmpl",
			"\"{{ `{{ .Cluster.Namespace }}` }}\"",
		)
		cm.Data["values.yaml"] = sourceString
		cm.Name = name

		finalContent := bytes.NewBuffer([]byte(fmt.Sprint("{{- if ", ifPipeline, " }}\n")))
		cmBytes, err := yaml.Marshal(&cm)
		if err != nil {
			return fmt.Errorf("failed to marshal %w", err)
		}
		_, err = finalContent.Write(cmBytes)
		if err != nil {
			return fmt.Errorf("failed to write %w", err)
		}
		_, err = finalContent.WriteString("{{- end -}}")
		if err != nil {
			return fmt.Errorf("failed to write to buffer %w", err)
		}
		destFile, err = os.Create(destPath)
		if err != nil {
			return fmt.Errorf("failed to truncate dest file %w", err)
		}
		defer destFile.Close()
		_, err = finalContent.WriteTo(destFile)
		if err != nil {
			return fmt.Errorf("failed to write to dest file %w", err)
		}
		return nil
	})
	return err
}

func extractContentAndName(node parse.Node, content *[]string, name, ifPipeline *string) {
	switch n := node.(type) {
	case *parse.ListNode:
		for _, node := range n.Nodes {
			extractContentAndName(node, content, name, ifPipeline)
		}
	case *parse.IfNode:
		// there should just be one in the templates here.
		*ifPipeline = n.BranchNode.Pipe.String()
		for _, node := range n.List.Nodes {
			extractContentAndName(node, content, name, ifPipeline)
		}
	case *parse.ActionNode:
		if *name == "" {
			*name = node.String()
		}
	default:
		*content = append(*content, node.String())
	}
}

func extractTemplateText(
	templateString string,
) (name, templateText, ifPipeline string, err error) {
	t, err := template.New("").Parse(templateString)
	if err != nil {
		return "", "", "", err
	}
	var content []string
	extractContentAndName(t.Root, &content, &name, &ifPipeline)
	return name, strings.Join(content, ""), ifPipeline, nil
}

func getHelmConfigMapFileName(filepath string) string {
	return strings.Replace(filepath, helmValuesFileName, helmValuesConfigMapName, 1)
}
