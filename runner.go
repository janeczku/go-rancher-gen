package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/rancher/go-rancher-metadata/metadata"
)

var (
	MetadataURL = "http://rancher-metadata"
)

type runner struct {
	Config  *Config
	Client  *metadata.Client
	Version string

	quitChan chan os.Signal
}

func NewRunner(conf *Config) (*runner, error) {
	u, _ := url.Parse(MetadataURL)
	u.Path = path.Join(u.Path, conf.MetadataVersion)

	log.Infof("Establishing connection to Rancher Metadata API: %s", u.String())

	client, err := metadata.NewClientAndWait(u.String())
	if err != nil {
		return nil, fmt.Errorf("Could not connect to Rancher Metadata API: %v", err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	return &runner{
		Config:   conf,
		Client:   client,
		Version:  "init",
		quitChan: c,
	}, nil
}

func (r *runner) Run() error {
	if r.Config.Onetime {
		log.Debug("Onetime mode")
		return r.poll()
	}

	log.Debugf("Polling metadata with %d secs interval", r.Config.Interval)
	ticker := time.NewTicker(time.Duration(r.Config.Interval) * time.Second)
	defer ticker.Stop()
	for {
		if err := r.poll(); err != nil {
			return err
		}

		log.Info("Waiting for changes in metadata...")
		select {
		case <-ticker.C:
		case signal := <-r.quitChan:
			log.Info("Exit requested by signal: ", signal)
			return nil
		}
	}
}

func (r *runner) poll() error {
	log.Debug("Checking if metadata has changed")
	newVersion, err := r.Client.GetVersion()
	if err != nil {
		log.Warnf("Failed to retrieve metadata version: %v", err)
		return nil
	}

	if r.Version == newVersion {
		log.Debug("No changes in metadata")
		return nil
	}

	log.Debug("Metadata version changed")
	log.Debugf("Old version: %s", r.Version)
	log.Debugf("New version: %s", newVersion)

	r.Version = newVersion
	ctx, err := r.createContext()
	if err != nil {
		log.Warnf("Failed to retrieve metadata: %v", err)
		return nil
	}

	tmplFuncs := newFuncMap(ctx)
	for _, tmpl := range r.Config.Templates {
		if err := r.processTemplate(tmplFuncs, tmpl); err != nil {
			return err
		}
	}

	if !r.Config.Onetime {
		log.Info("Processed templates. Waiting for changes in metadata...")
	}

	return nil
}

func (r *runner) processTemplate(funcs template.FuncMap, t Template) error {
	log.Debugf("Processing template %s", t.Source)

	tmplBytes, err := ioutil.ReadFile(t.Source)
	if err != nil {
		return fmt.Errorf("Reading template '%s': %v", t.Source, err)
	}
	name := filepath.Base(t.Source)
	newTemplate, err := template.New(name).Funcs(funcs).Parse(string(tmplBytes))
	if err != nil {
		return fmt.Errorf("Parsing template: %v", err)
	}

	buf := new(bytes.Buffer)
	if err := newTemplate.Execute(buf, nil); err != nil {
		return fmt.Errorf("Executing template: %v", err)
	}

	content := buf.Bytes()

	if t.Dest == "" {
		log.Debug("No destination file specified - printing generated content to STDOUT")
		os.Stdout.Write(content)
		return nil
	}

	changed, err := maybeUpdateDestination(content, t.Dest)
	if err != nil {
		return fmt.Errorf("Generating '%s': %v", t.Dest, err)
	}

	if !changed {
		log.Infof("Content of '%s' hasn't changed. Skipping notification.", t.Dest)
		return nil
	}

	log.Infof("Generated '%s'", t.Dest)

	return r.notify(t)
}

func (r *runner) createContext() (*TemplateContext, error) {

	log.Debug("Fetching metadata")

	metaServices, err := r.Client.GetServices()
	if err != nil {
		return nil, err
	}
	metaContainers, err := r.Client.GetContainers()
	if err != nil {
		return nil, err
	}
	metaHosts, err := r.Client.GetHosts()
	if err != nil {
		return nil, err
	}
	metaSelf, err := r.Client.GetSelfContainer()
	if err != nil {
		return nil, err
	}

	hosts := make([]Host, 0)
	for _, h := range metaHosts {
		host := Host{
			UUID:     h.UUID,
			Name:     h.Name,
			Address:  h.AgentIP,
			Hostname: h.Hostname,
			Labels:   LabelMap(h.Labels),
		}
		hosts = append(hosts, host)
	}

	containers := make([]Container, 0)
	for _, c := range metaContainers {
		container := Container{
			Name:    c.Name,
			Address: c.PrimaryIp,
			Stack:   c.StackName,
			Service: c.ServiceName,
			Health:  c.HealthState,
			Labels:  LabelMap(c.Labels),
		}
		for _, h := range hosts {
			if h.UUID == c.HostUUID {
				container.Host = h
				break
			}
		}
		containers = append(containers, container)
	}

	services := make([]Service, 0)
	for _, s := range metaServices {
		service := Service{
			Name:     s.Name,
			Stack:    s.StackName,
			Kind:     s.Kind,
			Vip:      s.Vip,
			Labels:   LabelMap(s.Labels),
			Metadata: MetadataMap(s.Metadata),
		}
		svcContainers := make([]Container, 0)
		for _, c := range containers {
			if c.Stack == s.StackName && c.Service == s.Name {
				svcContainers = append(svcContainers, c)
			}
		}
		service.Containers = svcContainers
		services = append(services, service)
	}

	self := Self{
		Stack:    metaSelf.StackName,
		Service:  metaSelf.ServiceName,
		HostUUID: metaSelf.HostUUID,
	}

	ctx := TemplateContext{
		Services:   services,
		Containers: containers,
		Hosts:      hosts,
		Self:       self,
	}

	return &ctx, nil
}

func (r *runner) notify(t Template) error {
	if t.NotifyCmd == "" {
		return nil
	}

	log.Infof("Running notify command '%s'", t.NotifyCmd)

	cmd := exec.Command("/bin/sh", "-c", t.NotifyCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error running notify command '%s': %v", t.NotifyCmd, err)
	}
	if t.NotifyOutput {
		for _, line := range strings.Split(string(out), "\n") {
			if line != "" {
				log.Infof("[%s]: %s", t.NotifyCmd, line)
			}
		}
	}

	return nil
}

func maybeUpdateDestination(content []byte, filePath string) (bool, error) {
	log.Debugf("Checking whether %s needs to be updated", filePath)

	lastMd5, err := computeMd5(filePath)
	if err != nil {
		return false, fmt.Errorf("Getting checksum: %v", err)
	}

	hash := md5.New()
	hash.Write(content)
	newMd5 := fmt.Sprintf("%x", hash.Sum(nil))

	log.Debugf("Last content checksum: %s", lastMd5)
	log.Debugf("New content checksum: %s", newMd5)

	if lastMd5 == newMd5 {
		return false, nil
	}

	fp, err := ioutil.TempFile(filepath.Dir(filePath), "rancher-gen")
	if err != nil {
		return false, fmt.Errorf("Creating temp file: %v", err)
	}

	defer func() {
		fp.Close()
		os.Remove(fp.Name())
	}()

	log.Debugf("Writing content to temp file %s", fp.Name())

	if _, err := fp.Write(content); err != nil {
		return false, fmt.Errorf("Writing temp file: %v", err)
	}

	// Copy file permissions/ownership
	if stat, err := os.Stat(filePath); err == nil {
		if err := fp.Chmod(stat.Mode()); err != nil {
			return false, fmt.Errorf("Setting file permissions: %v", err)
		}
		if os_stat, ok := stat.Sys().(*syscall.Stat_t); ok {
			if err := fp.Chown(int(os_stat.Uid), int(os_stat.Gid)); err != nil {
				return false, fmt.Errorf("Setting file ownership: %v", err)
			}
		}
	}

	log.Debugf("Copy temp file to destination %s", filePath)

	if err := os.Rename(fp.Name(), filePath); err != nil {
		return false, fmt.Errorf("Renaming temp file: %v", err)
	}

	return true, nil
}

func computeMd5(filePath string) (string, error) {
	if _, err := os.Stat(filePath); err != nil {
		return "", nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
