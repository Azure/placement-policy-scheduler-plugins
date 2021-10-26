package util

import "os"

func IntegrationEtcdServers() []string {
	if etcdURL, ok := os.LookupEnv("KUBE_INTEGRATION_ETCD_URL"); ok {
		return []string{etcdURL}
	}
	return []string{"http://127.0.0.1:2379"}
}
