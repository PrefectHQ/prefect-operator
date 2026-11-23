// Package clusterdomain finds the DNS domain of the cluster.
package clusterdomain

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
)

// ResolvConfPath is the resolver configuration that the kubelet writes in each
// container.
const ResolvConfPath = "/etc/resolv.conf"

// Resolve returns the DNS domain of the cluster: domain, or, when
// domain is empty, the one in the resolver configuration at path. It returns
// prefectiov1.DefaultClusterDomain if that file does not exist or names no
// cluster domain, as outside the cluster. It returns an error if the result is
// not a DNS name.
func Resolve(domain, path string) (string, error) {
	if domain == "" {
		content, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		domain = searched(string(content))
	}
	if domain == "" {
		domain = prefectiov1.DefaultClusterDomain
	}
	domain = strings.TrimSuffix(domain, ".")
	if errs := validation.IsDNS1123Subdomain(domain); len(errs) > 0 {
		return "", fmt.Errorf("the cluster domain %q is not a DNS name: %s", domain, strings.Join(errs, "; "))
	}
	return domain, nil
}

// searched returns the cluster domain from the search domains in
// resolvConf, the content of a resolv.conf file, or "". For pods that use the
// cluster's DNS, the kubelet starts the search domains with
// "<namespace>.svc.<cluster domain>", "svc.<cluster domain>" and
// "<cluster domain>". When resolvConf has more than one search line, the last
// one applies.
func searched(resolvConf string) string {
	domain := ""
	for line := range strings.Lines(resolvConf) {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "search" {
			continue
		}
		domain = ""
		searches := fields[1:]
		for i := range searches {
			searches[i] = strings.TrimSuffix(searches[i], ".")
		}
		if len(searches) < 3 || searches[1] != "svc."+searches[2] {
			continue
		}
		if namespace, ok := strings.CutSuffix(searches[0], "."+searches[1]); ok && !strings.Contains(namespace, ".") {
			domain = searches[2]
		}
	}
	return domain
}
