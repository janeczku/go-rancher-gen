package main

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
	"text/template"
	"net/url"
	"time"
	"reflect"
	"strconv"

	"github.com/Masterminds/sprig/v3"
	"github.com/wolfeidau/unflatten"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
)

func newFuncMap(ctx *TemplateContext) template.FuncMap {
	funcmap := template.FuncMap{
		// Utility funcs
		"base":         path.Base,
		"dir":          path.Dir,
		"env":          os.Getenv,
		"timestamp":    time.Now,
		"split":        strings.Split,
		"join":         strings.Join,
		"toUpper":      strings.ToUpper,
		"toLower":      strings.ToLower,
		"contains":     strings.Contains,
		"replace":      strings.Replace,
		"isJSONArray":  isJSONArray,
		"isJSONObject": isJSONObject,
		"unflatten": 		inflate,
		"yaml":					toYaml,
		"url": 					parseUrl,

		// Service funcs
		"self":              selfFunc(ctx),
		"host":              hostFunc(ctx),
		"hosts":             hostsFunc(ctx),
		"service":           serviceFunc(ctx),
		"services":          servicesFunc(ctx),
		"stack": 						 stackFunc(ctx),
		"stacks": 					 stacksFunc(ctx),
		"whereLabelExists":  whereLabelExists,
		"whereLabelEquals":  whereLabelEquals,
		"whereLabelMatches": whereLabelEquals,
		"groupByLabel":      groupByLabel,
	}

	for k, v := range sprig.TxtFuncMap() {
		funcmap[k] = v
  }

  return funcmap
}

// selfFunc returns the self object
func selfFunc(ctx *TemplateContext) func() (interface{}, error) {
	return func() (result interface{}, err error) {
		return ctx.Self, nil
	}
}

// serviceFunc returns a single service given a string argument in the form
// <service-name>[.<stack-name>].
func serviceFunc(ctx *TemplateContext) func(...string) (interface{}, error) {
	return func(s ...string) (result interface{}, err error) {
		result, err = ctx.GetService(s...)
		if _, ok := err.(NotFoundError); ok {
			log.Debug(err)
			return nil, nil
		}
		return
	}
}

// servicesFunc returns all available services, optionally filtered by stack
// name or label values.
func servicesFunc(ctx *TemplateContext) func(...string) (interface{}, error) {
	return func(s ...string) (interface{}, error) {
		return ctx.GetServices(s...)
	}
}

// stackFunc returns a single stack given a string argument in the form
// <service-name>.
func stackFunc(ctx *TemplateContext) func(...string) (interface{}, error) {
	return func(s ...string) (result interface{}, err error) {
		result, err = ctx.GetStack(s...)
		if _, ok := err.(NotFoundError); ok {
			log.Debug(err)
			return nil, nil
		}
		return
	}
}

// stacksFunc returns all available stacks.
func stacksFunc(ctx *TemplateContext) func() (interface{}, error) {
	return func() (interface{}, error) {
		return ctx.GetStacks()
	}
}


// hostFunc returns a single host given it's UUID.
func hostFunc(ctx *TemplateContext) func(...string) (interface{}, error) {
	return func(s ...string) (result interface{}, err error) {
		result, err = ctx.GetHost(s...)
		if _, ok := err.(NotFoundError); ok {
			log.Debug(err)
			return nil, nil
		}
		return
	}
}

// hostsFunc returns all available hosts, optionally filtered by label value.
func hostsFunc(ctx *TemplateContext) func(...string) (interface{}, error) {
	return func(s ...string) (interface{}, error) {
		return ctx.GetHosts(s...)
	}
}

// groupByLabel takes a label key and a slice of services or hosts and returns a map based
// on the values of the label.
//
// The map key is a string representing the label value. The map value is a
// slice of services or hosts that have the corresponding label value.
// Example:
//    {{range $labelValue, $containers := svc.Containers | groupByLabel "foo"}}
func groupByLabel(label string, in interface{}) (map[string][]interface{}, error) {
	m := make(map[string][]interface{})

	if in == nil {
		return m, fmt.Errorf("(groupByLabel) input is nil")
	}

	switch typed := in.(type) {
	case []*Service:
		for _, s := range typed {
			value, ok := s.Labels[label]
			if ok && len(value) > 0 {
				service := s
				m[value] = append(m[value], service)
			}
		}
	case []*Container:
		for _, c := range typed {
			value, ok := c.Labels[label]
			if ok && len(value) > 0 {
				container := c
				m[value] = append(m[value], container)
			}
		}
	case []*Host:
		for _, h := range typed {
			value, ok := h.Labels[label]
			if ok && len(value) > 0 {
				host := h
				m[value] = append(m[value], host)
			}
		}
	default:
		return m, fmt.Errorf("(groupByLabel) invalid input type %T", in)
	}

	return m, nil
}

func whereLabel(funcName string, in interface{}, label string, test func(string, bool) bool) ([]interface{}, error) {
	result := make([]interface{}, 0)
	if in == nil {
		return result, fmt.Errorf("(%s) input is nil", funcName)
	}
	if label == "" {
		return result, fmt.Errorf("(%s) label is empty", funcName)
	}

	switch typed := in.(type) {
	case []*Service:
		for _, s := range typed {
			value, ok := s.Labels[label]
			if test(value, ok) {
				service := s
				result = append(result, service)
			}
		}
	case []*Container:
		for _, c := range typed {
			value, ok := c.Labels[label]
			if test(value, ok) {
				container := c
				result = append(result, container)
			}
		}
	case []*Host:
		for _, h := range typed {
			value, ok := h.Labels[label]
			if test(value, ok) {
				host := h
				result = append(result, host)
			}
		}
	default:
		return result, fmt.Errorf("(%s) invalid input type %T", funcName, in)
	}

	return result, nil
}

// selects services or hosts from the input that have the given label
func whereLabelExists(label string, in interface{}) ([]interface{}, error) {
	return whereLabel("whereLabelExists", in, label, func(_ string, ok bool) bool {
		return ok
	})
}

// selects services or hosts from the input that have the given label and value
func whereLabelEquals(label, labelValue string, in interface{}) ([]interface{}, error) {
	return whereLabel("whereLabelEquals", in, label, func(value string, ok bool) bool {
		return ok && strings.EqualFold(value, labelValue)
	})
}

// selects services or hosts from the input that have the given label whose value matches the regex
func whereLabelMatches(label, pattern string, in interface{}) ([]interface{}, error) {
	rx, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	return whereLabel("whereLabelMatches", in, label, func(value string, ok bool) bool {
		return ok && rx.MatchString(value)
	})
}

func isJSONArray(in interface{}) bool {
	if _, ok := in.([]interface{}); ok {
		return true
	}
	return false
}

func isJSONObject(in interface{}) bool {
	if _, ok := in.(map[string]interface{}); ok {
		return true
	}
	return false
}

func toYaml(v interface{}) string {
	data, err := yaml.Marshal(v)
	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}
	return string(data)
}

func inflate(delimiter string, in interface{}) (map[string]interface{}, error) {
	msi := make(map[string]interface{})
	iter := reflect.ValueOf(in).MapRange()
	for iter.Next() {
		key := iter.Key().String()
		msi[key] = iter.Value().Interface()
	}

	return unflatten.Unflatten(msi, func(k string) []string { return strings.Split(k, delimiter) }), nil
}

func parseUrl(urlStr string) *ParsedUrl {
	parseable := urlStr
	if ! (strings.HasPrefix(urlStr, "//") || strings.Contains(urlStr, "://")) {
		parseable = "//" + urlStr
	}

	parsed, err := url.Parse(parseable)
	if err != nil { return nil }

	obj := ParsedUrl{
		Scheme: 		parsed.Scheme,
		Host: 			parsed.Hostname(),
		Path: 			parsed.Path,
	}

	port, ok := strconv.Atoi(parsed.Port())
	if ok == nil { obj.Port = port }

	if parsed.User != nil {
		obj.Username = parsed.User.Username()
		password, exists := parsed.User.Password()
		if exists { obj.Password = password }
	}

	return &obj
}
