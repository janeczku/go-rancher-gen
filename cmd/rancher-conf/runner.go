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

type runner struct {
	Config  *Config
	Client  metadata.Client
	Version string

	quitChan chan os.Signal
}

func NewRunner(conf *Config) (*runner, error) {
	u, _ := url.Parse(conf.MetadataUrl)
	u.Path = path.Join(u.Path, conf.MetadataVersion)

	log.Infof("Initializing Rancher Metadata client (version %s)", conf.MetadataVersion)

	client, err := metadata.NewClientAndWait(u.String())
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize Rancher Metadata client: %v", err)
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
	if r.Config.OneTime {
		log.Info("Processing all templates once.")
		return r.poll()
	}

	log.Info("Polling Metadata with %d second interval", r.Config.Interval)
	ticker := time.NewTicker(time.Duration(r.Config.Interval) * time.Second)
	defer ticker.Stop()
	for {
		if err := r.poll(); err != nil {
			log.Error(err)
		}

		select {
		case <-ticker.C:
		case signal := <-r.quitChan:
			log.Info("Exit requested by signal: ", signal)
			return nil
		}
	}
}

func (r *runner) poll() error {
	log.Debug("Checking for metadata change")
	newVersion, err := r.Client.GetVersion()
	if err != nil {
		time.Sleep(time.Second * 2)
		return fmt.Errorf("Failed to get Metadata version: %v", err)
	}

	if r.Version == newVersion {
		log.Debug("No changes in Metadata")
		return nil
	}

	log.Debugf("Old version: %s, New Version: %s", r.Version, newVersion)

	r.Version = newVersion
	ctx, err := r.createContext()
	if err != nil {
		time.Sleep(time.Second * 2)
		return fmt.Errorf("Failed to create context from Rancher Metadata: %v", err)
	}

	tmplFuncs := newFuncMap(ctx)
	for _, tmpl := range r.Config.Templates {
		if err := r.processTemplate(tmplFuncs, tmpl); err != nil {
			return err
		}
	}

	if r.Config.OneTime {
		log.Info("All templates processed. Exiting.")
	} else {
		log.Info("All templates processed. Waiting for changes in Metadata...")
	}

	return nil
}

func (r *runner) processTemplate(funcs template.FuncMap, t Template) error {
	log.Debugf("Processing template %s for destination %s", t.Source, t.Dest)
	if _, err := os.Stat(t.Source); os.IsNotExist(err) {
		log.Fatalf("Template '%s' is missing", t.Source)
	}

	tmplBytes, err := ioutil.ReadFile(t.Source)
	if err != nil {
		log.Fatalf("Could not read template '%s': %v", t.Source, err)
	}

	name := filepath.Base(t.Source)
	newTemplate, err := template.New(name).Funcs(funcs).Parse(string(tmplBytes))
	if err != nil {
		log.Fatalf("Could not parse template '%s': %v", t.Source, err)
	}

	buf := new(bytes.Buffer)
	if err := newTemplate.Execute(buf, nil); err != nil {
		log.Fatalf("Could not render template: '%s': %v", t.Source, err)
	}

	content := buf.Bytes()

	if t.Dest == "" {
		log.Debug("No destination specified. Printing to StdOut")
		os.Stdout.Write(content)
		return nil
	}

	log.Debug("Checking whether content has changed")
	same, err := sameContent(content, t.Dest)
	if err != nil {
		return fmt.Errorf("Could not compare content for %s: %v", t.Dest, err)
	}

	if same {
		log.Debugf("Destination %s is up to date", t.Dest)
		return nil
	}

	log.Debug("Creating staging file")
	stagingFile, err := createStagingFile(content, t.Dest)
	if err != nil {
		return err
	}

	defer os.Remove(stagingFile)

	if t.CheckCmd != "" {
		if err := check(t.CheckCmd, stagingFile); err != nil {
			return fmt.Errorf("Check command failed: %v", err)
		}
	}

	log.Debugf("Writing destination")
	if err = copyStagingToDestination(stagingFile, t.Dest); err != nil {
		return fmt.Errorf("Could not write destination file %s: %v", t.Dest, err)
	}

	log.Info("Destination file %s has been updated", t.Dest)

	if t.NotifyCmd != "" {
		if err := notify(t.NotifyCmd, t.NotifyOutput); err != nil {
			return fmt.Errorf("Notify command failed: %v", err)
		}
	}

	return nil
}

func copyStagingToDestination(stagingPath, destPath string) error {
	err := os.Rename(stagingPath, destPath)
	if err == nil {
		return nil
	}

	if !strings.Contains(err.Error(), "device or resource busy") {
		return err
	}

	// A 'device busy' error could mean that the files live in
	// different mounts. Try to read the staging file and write
	// it's content to the destination file.
	log.Debugf("Failed to rename staging file: %v", err)

	content, err := ioutil.ReadFile(stagingPath)
	if err != nil {
		return err
	}

	sfi, err := os.Stat(stagingPath)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(destPath, content, sfi.Mode()); err != nil {
		return err
	}

	if os_stat, ok := sfi.Sys().(*syscall.Stat_t); ok {
		if err := os.Chown(destPath, int(os_stat.Uid), int(os_stat.Gid)); err != nil {
			return err
		}
	}

	return nil
}

func (r *runner) createContext() (*TemplateContext, error) {
	log.Debug("Fetching Metadata")

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

	self := Self{
		Stack: metaSelf.StackName,
	}

	hosts := make([]*Host, 0)
	for _, h := range metaHosts {
		host := Host{
			UUID:     h.UUID,
			Name:     h.Name,
			Address:  h.AgentIP,
			Hostname: h.Hostname,
			Labels:   LabelMap(h.Labels),
		}
		hosts = append(hosts, &host)

		if h.UUID == metaSelf.HostUUID {
			self.Host = &host
		}
	}

	services := make([]*Service, 0)
	for _, s := range metaServices {
		service := Service{
			Name:       s.Name,
			Stack:      s.StackName,
			Kind:       s.Kind,
			Vip:        s.Vip,
			Fqdn:       s.Fqdn,
			Labels:     LabelMap(s.Labels),
			Metadata:   MetadataMap(s.Metadata),
			Containers: make([]*Container, 0),
			Ports:      parseServicePorts(s.Ports),
		}

		services = append(services, &service)

		if s.Name == metaSelf.ServiceName && s.StackName == metaSelf.StackName {
			self.Service = &service
		}
	}

	sidekickMap := make(map[string]*Container)
	containers := make([]*Container, 0)
	for _, c := range metaContainers {
		labels := LabelMap(c.Labels)

		container := Container{
			UUID:      c.UUID,
			Name:      c.Name,
			Address:   c.PrimaryIp,
			Stack:     c.StackName,
			Health:    c.HealthState,
			State:     c.State,
			Labels:    labels,
			Sidekicks: make([]*Container, 0),
		}

		for _, h := range hosts {
			if h.UUID == c.HostUUID {
				host := h
				container.Host = host
				break
			}
		}

		for _, s := range services {
			if s.Name == c.ServiceName && s.Stack == c.StackName {
				service := s
				service.Containers = append(service.Containers, &container)
				container.Service = service
				break
			}
		}

		deployment := labels.GetValue("io.rancher.service.deployment.unit")
		launchConfig := labels.GetValue("io.rancher.service.launch.config")
		if launchConfig == "io.rancher.service.primary.launch.config" {
			sidekickMap[deployment] = &container
		}

		if c.UUID == metaSelf.UUID {
			self.Container = &container
		}

		containers = append(containers, &container)
	}

	for _, c := range containers {
		launchConfig := c.Labels.GetValue("io.rancher.service.launch.config")
		deployment := c.Labels.GetValue("io.rancher.service.deployment.unit")
		parent, hasParent := sidekickMap[deployment]
		if hasParent && launchConfig != "io.rancher.service.primary.launch.config" {
			container := c
			container.Parent = parent
			container.Service.Parent = parent.Service
			parent.Sidekicks = append(parent.Sidekicks, container)
		}
	}

	log.Debugf("Finished building context")

	ctx := TemplateContext{
		Hosts:      hosts,
		Services:   services,
		Containers: containers,
		Self:       &self,
	}

	return &ctx, nil
}

// converts Metadata.Service.Ports string slice to a ServicePort slice
func parseServicePorts(ports []string) []ServicePort {
	var ret []ServicePort
	for _, port := range ports {
		if parts := strings.Split(port, ":"); len(parts) == 2 {
			public := parts[0]
			if parts_ := strings.Split(parts[1], "/"); len(parts_) == 2 {
				ret = append(ret, ServicePort{
					PublicPort:   public,
					InternalPort: parts_[0],
					Protocol:     parts_[1],
				})
				continue
			}
		}
		log.Warnf("Unexpected format of service port: %s", port)
	}

	return ret
}

func check(command, filePath string) error {
	command = strings.Replace(command, "{{staging}}", filePath, -1)
	log.Debugf("Running check command '%s'", command)
	cmd := exec.Command("/bin/sh", "-c", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logCmdOutput(command, out)
		return err
	}

	log.Debugf("Check cmd output: %q", string(out))
	return nil
}

func notify(command string, verbose bool) error {
	log.Infof("Executing notify command '%s'", command)
	cmd := exec.Command("/bin/sh", "-c", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logCmdOutput(command, out)
		return err
	}

	if verbose {
		logCmdOutput(command, out)
	}

	log.Debugf("Notify cmd output: %q", string(out))
	return nil
}

func logCmdOutput(command string, output []byte) {
	for _, line := range strings.Split(string(output), "\n") {
		if line != "" {
			log.Infof("[%s]: %q", command, line)
		}
	}
}

func sameContent(content []byte, filePath string) (bool, error) {
	fileMd5, err := computeFileMd5(filePath)
	if err != nil {
		return false, fmt.Errorf("Could not calculate checksum for %s: %v",
			filePath, err)
	}

	hash := md5.New()
	hash.Write(content)
	contentMd5 := fmt.Sprintf("%x", hash.Sum(nil))

	log.Debugf("Checksum content: %s, checksum file: %s",
		contentMd5, fileMd5)

	if fileMd5 == contentMd5 {
		return true, nil
	}

	return false, nil
}

func computeFileMd5(filePath string) (string, error) {
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

func createStagingFile(content []byte, destFile string) (string, error) {
	fp, err := ioutil.TempFile(filepath.Dir(destFile), "."+filepath.Base(destFile)+"-")
	if err != nil {
		return "", fmt.Errorf("Could not create staging file for %s: %v", destFile, err)
	}

	log.Debugf("Created staging file %s", fp.Name())

	onErr := func() {
		fp.Close()
		os.Remove(fp.Name())
	}

	if _, err := fp.Write(content); err != nil {
		onErr()
		return "", fmt.Errorf("Could not write staging file for %s: %v", destFile, err)
	}

	log.Debug("Copying file permissions and owner from destination")
	if stat, err := os.Stat(destFile); err == nil {
		if err := fp.Chmod(stat.Mode()); err != nil {
			onErr()
			return "", fmt.Errorf("Failed to copy permissions from %s: %v", destFile, err)
		}
		if os_stat, ok := stat.Sys().(*syscall.Stat_t); ok {
			if err := fp.Chown(int(os_stat.Uid), int(os_stat.Gid)); err != nil {
				onErr()
				return "", fmt.Errorf("Failed to copy ownership: %v", err)
			}
		}
	}

	fp.Close()
	return fp.Name(), nil
}
