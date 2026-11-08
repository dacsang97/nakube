package cluster

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ForContext creates a read session without switching the desktop or writing its
// selected-context file. It shares only the immutable kubeconfig loading rules.
func (s *Session) ForContext(name string) (*Session, error) {
	s.mu.Lock()
	if s.rules == nil {
		s.rules = clientcmd.NewDefaultClientConfigLoadingRules()
	}
	rules := s.rules
	s.mu.Unlock()
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{CurrentContext: name})
	raw, err := loader.RawConfig()
	if err != nil {
		return nil, fmt.Errorf("kubeconfig could not be read")
	}
	if _, ok := raw.Contexts[name]; !ok {
		return nil, fmt.Errorf("kube context %q is not in the kubeconfig", name)
	}
	cfg, err := loader.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("kube context could not be loaded")
	}
	cfg.QPS, cfg.Burst = clientQPS, clientBurst
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("kubernetes client could not be built")
	}
	return &Session{name: name, rules: rules, cfg: cfg, cs: cs}, nil
}
