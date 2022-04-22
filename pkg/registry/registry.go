package registry

type RegisterRequest struct {
	Name string `json:"name"`
	Addr string `json:"addr"`
}

type UnregisterRequest struct {
	Name string `json:"name"`
}

type Registry struct {
	AddrByName map[string]string `json:"addrByName"`
}

func New() *Registry {
	return &Registry{
		AddrByName: make(map[string]string),
	}
}

func (reg *Registry) Register(name string, addr string) {
	reg.AddrByName[name] = addr
}

func (reg *Registry) Unregister(name string) {
	delete(reg.AddrByName, name)
}
