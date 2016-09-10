package main

// Service represents a Rancher service.
type Service struct {
	Name       string
	Stack      string
	Kind       string // service, loadBalancerService
	Vip        string
	Fqdn       string
	Ports      []ServicePort
	Labels     LabelMap
	Metadata   MetadataMap
	Containers []Container
}

// Container represents a container belonging to a Rancher Service.
type Container struct {
	Name    string
	Address string
	Stack   string
	Service string
	Health  string
	State   string
	Labels  LabelMap
	Host    Host
}

// Host represents a Rancher Host.
type Host struct {
	UUID     string
	Name     string
	Address  string
	Hostname string
	Labels   LabelMap
}

// Self contains information about the container running this application.
type Self struct {
	Stack    string
	Service  string
	HostUUID string
}

// ServicePort represents a port exposed by a service
type ServicePort struct {
	PublicPort   string
	InternalPort string
	Protocol     string
}

// LabelMap contains the labels of a service or host.
type LabelMap map[string]string

// Exists returns true if the Labels contain the given key.
func (l LabelMap) Exists(key string) bool {
	_, ok := l[key]

	return ok
}

// Value returns the value of the given label key.
func (l LabelMap) GetValue(key string, v ...string) string {
	if val, ok := l[key]; ok && len(val) > 0 {
		return val
	}

	if len(v) > 0 {
		return v[0]
	}

	return ""
}

// MetadataMap contains the metadata of a service.
type MetadataMap map[string]interface{}

// Exists returns true if the metadata contains the given key.
func (m MetadataMap) Exists(key string) bool {
	_, ok := m[key]

	return ok
}

// Value returns the value of the given metadata key.
func (m MetadataMap) GetValue(key string, v ...interface{}) interface{} {
	if val, ok := m[key]; ok {
		return val
	}

	if len(v) > 0 {
		return v[0]
	}

	return ""
}
