package httpproxyconfig

import (
	"testing"

	. "github.com/onsi/gomega"

	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
)

func newTestGenerator() *systemdConfigGenerator {
	return &systemdConfigGenerator{
		template: templates.Lookup("systemd.conf.tmpl"),
	}
}

func TestGenerate(t *testing.T) {
	g := NewWithT(t)

	content, err := newTestGenerator().Generate(HTTPProxyVariables{
		HTTP:  "http://example.com",
		HTTPS: "https://example.com",
		NO: []string{
			"https://no-proxy.example.com",
		},
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(content).To(And(
		ContainSubstring(`Environment="HTTP_PROXY=http://example.com"`),
		ContainSubstring(`Environment="http_proxy=http://example.com"`),
		ContainSubstring(`Environment="HTTPS_PROXY=https://example.com"`),
		ContainSubstring(`Environment="https_proxy=https://example.com"`),
		ContainSubstring(`Environment="NO_PROXY=https://no-proxy.example.com"`),
		ContainSubstring(`Environment="no_proxy=https://no-proxy.example.com"`),
	))
}

func TestGenerate_OnlyHTTP(t *testing.T) {
	g := NewWithT(t)

	content, err := newTestGenerator().Generate(HTTPProxyVariables{
		HTTP: "http://example.com",
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(content).To(And(
		ContainSubstring(`Environment="HTTP_PROXY=http://example.com"`),
		ContainSubstring(`Environment="http_proxy=http://example.com"`),
		Not(ContainSubstring(`Environment="HTTPS_PROXY=`)),
		Not(ContainSubstring(`Environment="https_proxy=`)),
		Not(ContainSubstring(`Environment="NO_PROXY=`)),
		Not(ContainSubstring(`Environment="no_proxy=`)),
	))
}

func TestGenerate_OnlyHTTPS(t *testing.T) {
	g := NewWithT(t)

	content, err := newTestGenerator().Generate(HTTPProxyVariables{
		HTTPS: "https://example.com",
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(content).To(And(
		Not(ContainSubstring(`Environment="HTTP_PROXY=http://example.com"`)),
		Not(ContainSubstring(`Environment="http_proxy=http://example.com"`)),
		ContainSubstring(`Environment="HTTPS_PROXY=https://example.com"`),
		ContainSubstring(`Environment="https_proxy=https://example.com"`),
		Not(ContainSubstring(`Environment="NO_PROXY=https://no-proxy.example.com"`)),
		Not(ContainSubstring(`Environment="no_proxy=https://no-proxy.example.com"`)),
	))
}

func TestGenerate_OnlyNoProxy(t *testing.T) {
	g := NewWithT(t)

	content, err := newTestGenerator().Generate(HTTPProxyVariables{
		NO: []string{"https://no-proxy.example.com"},
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(content).To(And(
		Not(ContainSubstring(`Environment="HTTP_PROXY="`)),
		Not(ContainSubstring(`Environment="http_proxy="`)),
		Not(ContainSubstring(`Environment="HTTPS_PROXY="`)),
		Not(ContainSubstring(`Environment="https_proxy=`)),
		ContainSubstring(`Environment="NO_PROXY=https://no-proxy.example.com"`),
		ContainSubstring(`Environment="no_proxy=https://no-proxy.example.com"`),
	))
}

func TestGenerate_NoProxyMultipleURLs(t *testing.T) {
	g := NewWithT(t)

	content, err := newTestGenerator().Generate(HTTPProxyVariables{
		NO: []string{
			"https://no-proxy.example.com",
			"https://no-proxy-1.example.com",
		},
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(content).To(And(
		ContainSubstring(`Environment="NO_PROXY=https://no-proxy.example.com,https://no-proxy-1.example.com"`),
		ContainSubstring(`Environment="no_proxy=https://no-proxy.example.com,https://no-proxy-1.example.com"`),
	))
}

func TestAddSystemdFiles(t *testing.T) {
	g := NewWithT(t)

	dst := []bootstrapv1.File{}
	g.Expect(newTestGenerator().AddSystemdFiles(HTTPProxyVariables{}, dst)).To(HaveLen(2))
}
