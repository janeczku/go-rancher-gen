package main

import "github.com/finboxio/go-rancher-metadata/metadata"

type Self struct {
  Stack     *Stack
  Service   *Service
  Container *Container
  Host      *Host
}

type Stack struct {
  metadata.Stack
  Services      []*Service
}

// Host represents a Rancher Host.
type Host struct {
  metadata.Host

  Labels        LabelMap

  Containers []*Container
}

// Service represents a Rancher service.
type Service struct {
  metadata.Service

  Sidekicks     []*Service
  Containers    []*Container
  Ports         []ServicePort
  Labels        LabelMap
  Links         LabelMap
  Metadata      MetadataMap

  Primary       bool
  Sidekick      bool
  Stack         *Stack
  Parent        *Service
}

// Container represents a container belonging to a Rancher Service.
type Container struct {
  metadata.Container

  Ports         []ServicePort
  Labels        LabelMap
  Links         LabelMap

  Primary       bool
  Sidekick      bool
  Service       *Service
  Host          *Host
  Parent        *Container
  Sidekicks     []*Container
}

// ServicePort represents a port exposed by a service
type ServicePort struct {
  BindAddress  string
  PublicPort   string
  InternalPort string
  Protocol     string
}

type ParsedUrl struct {
  Scheme      string
  Host        string
  Port        int
  Path        string
  Username    string
  Password    string
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
