package injection

import (
	"sync"

	"knative.dev/pkg/injection"
)

var (
	Default = &impl{
		pkgInjector: injection.Default,
	}
)

type impl struct {
	m sync.RWMutex

	pkgInjector injection.Interface
	crds        []string
}

func (i *impl) RegisterInformer(ii injection.InformerInjector, crd string) {
	i.m.Lock()
	defer i.m.Unlock()

	i.crds = append(i.crds, crd)
	i.pkgInjector.RegisterInformer(ii)
}

func (i *impl) GetCRDs() []string {
	i.m.RLock()
	defer i.m.RUnlock()

	// Copy the slice before returning.
	return append(i.crds[:0:0], i.crds...)
}
