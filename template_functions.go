package main

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
	"text/template"
	"time"
)

func newFuncMap(ctx *TemplateContext) template.FuncMap {
	return template.FuncMap{
		// Utility funcs
		"base":      path.Base,
		"dir":       path.Dir,
		"env":       os.Getenv,
		"timestamp": time.Now,
		"split":     strings.Split,
		"join":      strings.Join,
		"toUpper":   strings.ToUpper,
		"toLower":   strings.ToLower,
		"contains":  strings.Contains,
		"replace":   strings.Replace,

		// Service funcs
		"host":              hostFunc(ctx),
		"hosts":             hostsFunc(ctx),
		"service":           serviceFunc(ctx),
		"services":          servicesFunc(ctx),
		"whereLabelExists":  whereLabelExists,
		"whereLabelEquals":  whereLabelEquals,
		"whereLabelMatches": whereLabelEquals,
		"groupByLabel":      groupByLabel,
	}
}

// serviceFunc returns a single service given a string argument in the form
// <service-name>[.<stack-name>].
func serviceFunc(ctx *TemplateContext) func(...string) (Service, error) {
	return func(s ...string) (Service, error) {
		return ctx.GetService(s...)
	}
}

// servicesFunc returns all available services, optionally filtered by stack
// name or label values.
func servicesFunc(ctx *TemplateContext) func(...string) ([]Service, error) {
	return func(s ...string) ([]Service, error) {
		return ctx.GetServices(s...)
	}
}

// hostFunc returns a single host given it's UUID.
func hostFunc(ctx *TemplateContext) func(...string) (Host, error) {
	return func(s ...string) (Host, error) {
		return ctx.GetHost(s...)
	}
}

// hostsFunc returns all available hosts, optionally filtered by label value.
func hostsFunc(ctx *TemplateContext) func(...string) ([]Host, error) {
	return func(s ...string) ([]Host, error) {
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
	case []Service:
		for _, s := range typed {
			value, ok := s.Labels[label]
			if ok && len(value) > 0 {
				m[value] = append(m[value], s)
			}
		}
	case []Container:
		for _, c := range typed {
			value, ok := c.Labels[label]
			if ok && len(value) > 0 {
				m[value] = append(m[value], c)
			}
		}
	case []Host:
		for _, h := range typed {
			value, ok := h.Labels[label]
			if ok && len(value) > 0 {
				m[value] = append(m[value], h)
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
	case []Service:
		for _, s := range typed {
			value, ok := s.Labels[label]
			if test(value, ok) {
				result = append(result, s)
			}
		}
	case []Container:
		for _, c := range typed {
			value, ok := c.Labels[label]
			if test(value, ok) {
				result = append(result, c)
			}
		}
	case []Host:
		for _, s := range typed {
			value, ok := s.Labels[label]
			if test(value, ok) {
				result = append(result, s)
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
