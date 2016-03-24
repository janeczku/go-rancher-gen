Rancher Template
===============
[![Latest Version](https://img.shields.io/badge/release-0.1.0-green.svg?style=flat)][release] [![CircleCI](https://img.shields.io/circleci/project/janeczku/rancher-template.svg)][circleci] [![Docker Pulls](https://img.shields.io/docker/pulls/janeczku/rancher-template.svg)](https://hub.docker.com/r/janeczku/rancher-template/) ![License MIT](https://img.shields.io/badge/license-MIT-blue.svg?style=flat)

[release]: https://github.com/janeczku/rancher-template/releases
[circleci]: https://circleci.com/gh/janeczku/rancher-template

`rancher-template` is a file generator providing a template syntax with **Rancher Service Discovery** as its first-class citizen.

It renders the specified templates to the local filesystem using service, container and host information provided by [Rancher's Metadata Service](http://docs.rancher.com/rancher/metadata-service/). After a target file has been updated it can optionally execute an arbitrary command (e.g. to trigger a reload of the application consuming the file).

Installation
------------

`rancher-template` can be bundled with the application consuming the generated files or run in it's own container as a [service sidekick](http://docs.rancher.com/rancher/rancher-compose/#sidekicks).

### Bundled Application Image

Download the binary from the [release page][release].
Add the binary to your Docker image and provide a mechanism that executes `rancher-template` on container start, waits until it has generated the target files and then launches the application consuming the files. This functionality could be provided by a bash script that is executed as image `ENTRYPOINT`. Using a container-level process supervisor (e.g. [S6-overlay](https://github.com/just-containers/s6-overlay)) is another option.

### Sidekick Container
Create a new Docker image using `janeczku/rancher-template:latest` as base. Add the configuration file and the template(s) to the image. Expose the destination directory of the generated files as `VOLUME`. Pass the configuration file path to `rancher-template` by specifying the corresponding flag in the `CMD` parameter.

##### Example sidekick image

```DOCKERFILE
FROM janeczku/rancher-template:v0.1.0
COPY config.toml /etc/rancher-template/
COPY nginx.tmpl /etc/rancher-template/
VOLUME /etc/nginx
CMD ["--config", "/etc/rancher-template/config.toml"]
```

##### Example Rancher Compose file

```YAML
nginx:
  image: nginx:latest
  volumes_from:
  - template-sidekick
  labels:
    io.rancher.sidekicks: template-sidekick
template-sidekick:
  image: janeczku/rancher-template:latest
```

Usage
------------

### Command Line

``` rancher-template [options] template [destination]```

#### options

|       Flag         |            Description         |
| ------------------ | ------------------------------ |
| `config`           | Path to a configuration file. Values specified on the CLI take precedence over values specified in the configuration file.
| `metadata-version` | Metadata version string used when querying the Metadata Service. Default: `latest`.
| `interval`         | Interval for polling the Metadata Service for changes (in seconds). Default: `60`
| `onetime`          | Generate all files once and exit. Default: `false`
| `log-level`        | Verbosity of log output. Valid values: "debug", "info", "warn", and "error". Default: `info`.
| `notify-cmd`       | Optional command to run after the destination file has been updated.
| `notify-output`    | Log the result of the notify command
| `version`          | Print the program version and exit. 

#### template
Path to the template file

#### destination
Path to the destination file. If omitted the generated content will be printed to STDOUT.

### Configuration file

Passing a configuration file to `rancher-template` allows for multiple templates to be defined (as opposed to just one via the CLI). The configuration file uses the [TOML](https://github.com/toml-lang/toml) format. It supports the same options as the CLI in addition to one or multiple `template` sections. An example config is available [here](examples/config.toml.sample). 

Templating Language
------------
Template files are written in the [Go Template](http://golang.org/pkg/text/template/) format.

In addition to the built-in functions, `rancher-template` exposes additional functions and methods for discovery of Rancher services, containers and hosts.

### Service Discovery Object Types

```go
type Service struct {
	Name        string
	Stack       string
	Kind        string
	Vip         string
	Labels      LabelMap
	Metadata    MetadataMap
	Containers  []Container
}

type Container struct {
	Name        string
	Address     string
	Stack       string
	Service     string
	Health      string
	Host        Host
}

type Host struct {
	UUID        string
	Name        string
	Address     string
	Labels      LabelMap
}
```

The `LabelMap` and `MetadataMap` types implement methods for easily checking the existence of specific keys and accessing their values:

##### `Labels.Exists(key string) bool`   
Returns true if the given label key exists in the map.

##### `Labels.GetValue(key, default string) string`  
Returns the value of the given label key. The function accepts an optional default value that is returned when the key doesn't exist or is set to an empty string.

##### `Metadata.Exists(key string) bool`   
Returns true if the given metadata key exists in the map.

##### `Metadata.GetValue(key, default interface{}) interface{}`  
Returns the value of the given label key. The function accepts an optional default value that is returned when the key doesn't exist.

**Examples**:

Check if the label exists:

```liquid
{{range services}}
{{if .Labels.Exists "foo"}}
{{do something}}
{{end}}
{{end}}
```

Get the value of a Metadata key:

```liquid
{{range services}}
Metadata foo: {{.Metadata.GetValue "foo"}}
{{end}}
```
Using a default value:

```liquid
{{range services}}
Label foo: {{.Labels.GetValue "foo" "default value"}}
{{end}}
```

### Service Discovery Functions

#### `host`

Lookup a specific host

**Optional argument**   
UUID *string*    
**Return Type**   
`Host`

If the argument is omitted the local host is returned:

```liquid
{{host}}
```

#### `hosts`

Lookup hosts

**Optional parameters**   
labelSelector *string*    
**Returned Type**   
`[]Host`

The function returns a slice of `Host` which can be used for ranging in a template:

```liquid
{{range hosts}}
host {{.Name}} {{.Address}}
{{end}}
```

which would produce something like:

```text
host aws-sm-01 148.210.10.10
host aws-sm-02 148.210.10.11
```

One or multiple label selectors can be passed as arguments to limit the result to hosts with matching labels. The syntax of the label selector is `@label-key=label-value`.

The following function returns only hosts that have a label "foo" with the value "bar":

```liquid
{{hosts "@foo=bar"}}
```

The label selector syntax supports a regex pattern on it's right side. E.g. to lookup hosts that have a specific label regardless of the value:

```liquid
{{hosts "@foo=.*"}}
```

If the argument is omitted all hosts are returned:

```liquid
{{hosts}}
```

#### `service`

Lookup a specific service

**Optional parameter**   
serviceIdentifier *string*       
**Returned Type**   
`Service`

The function returns a `Service` struct. You can use the `Containers` field for ranging over all containers belonging to the service:

```liquid
{{service "web.production"}}
{{range .Containers}}
http://{{.Address}}:9090
{{end}}
```

which would produce something like:

```text
http://10.12.20.111:9090
http://10.12.20.122:9090
```

The syntax o the serviceIdentifier parameter is `service-name[.stack-name]`:

```liquid
{{service "web.production"}}
```

If the stack name is omitted the service is looked up in the local stack:

```liquid
{{service "web"}}
```

If no argument is given the local service is returned:

```liquid
{{service}}
```

#### `services`

Lookup services matching the given stack and label selectors

**Optional parameters**   
stackSelector *string*   
labelSelector *string*     
**Return Type**   
`[]Service`

Just like with the `hosts` function multiple label selectors can be passed to select services with matching labels:

```liquid
{{services "@foo=bar"}}
```

The stack selector parameter uses the syntax `.stack-name`:

```liquid
{{services ".production"}}
```

Stack and label selectors can be combined like this:

```liquid
{{services ".production" "@foo=bar"}}
```

If arguments are omitted then all services are returned:

```liquid
{{services}}
```

### Helper Functions and Pipes

#### `whereLabelExists`

Filter a slice of hosts, services or containers returning the items that have the given label key.

**Parameters**     
labelKey *string*    
input *[]Host, []Service or []Container*   
**Return Type**   
same as input

#### `whereLabelEquals`

Filter a slice of hosts, services or containers returning the items that have the given label key and value.

**Arguments**   
labelKey *string*    
labelValue *string*    
input *[]Host, []Service or []Container*   
**Return Type**   
same as input

```liquid
{{range $container := whereLabelEquals "foo" "bar" $svc.Containers}}
{{do something}}
{{end}}
```

#### `whereLabelMatches`

Filter a slice of hosts, services or containers returning the items that have the given label and a value matching the regex pattern.

**Arguments**   
labelKey *string*    
regexPattern *string*     
input *[]Host, []Service or []Container*    
**Return Type**   
same as input

#### `groupByLabel`

This function takes a slice of hosts, services or containers and groups the items by their value of the given label. It returns a map with label values as key and a slice of corresponding elements items as value.

**Arguments**   
label-key *string*  
input *[]Host,[]Service,[]Container*       
**Return Type**   
map[string][]Host/[]Service/[]Container

```liquid
{{range $labelValue, $hosts := hosts | groupByLabel "foo"}}
{{$labelValue}}
{{range $hosts}}
IP: {{.Address}}
{{end}}
{{end}}
```

##### `base`

Alias for the path.Base function

```liquid
filename: {{svc.Metadata.GetValue "targetPath" | base}}
```

See Go's [path.Base()](https://golang.org/pkg/path/#Base) for more information.

##### `dir`

Alias for the path.Dir function

See Go's [path.Dir()](https://golang.org/pkg/path/#Dir) for more information.

##### `env`

Returns the value of the given environment variable or an empty string if the variable isn't set

```liquid
{{env "FOO_VAR"}}
```

##### `timestamp`

Alias for time.Now

```liquid
# Generated by rancher-template {{timestamp}}
```

The timestamp can be formatted as required by invoking the `Format` method:

```liquid
# Generated by rancher-template {{timestamp.Format "Jan 2, 2006 15:04"}}
```

See Go's [time.Format()](http://golang.org/pkg/time/#Time.Format) for more information about formatting the date according to the layout of the reference time.

##### `split`

Alias for strings.Split

```liquid
{{$items := split $someString ":"}}
```

See Go's [strings.Split()](http://golang.org/pkg/strings/#Split) for more information.

##### `join`

Alias for strings.Join    
Takes the given slice of strings as a pipe and joins them on the provided string:

```liquid
{{$items | join ","}}
```

See Go's [strings.Join()](http://golang.org/pkg/strings/#Join) for more information.

##### `toLower`

Alias for strings.ToLower    
Takes the argument as a string and converts it to lowercase.

```liquid
{{$svc.Metadata.GetValue "foo" | toLower}}
```

See Go's [strings.ToLower()](http://golang.org/pkg/strings/#ToLower) for more information.

##### `toUpper`

Alias for strings.ToUpper    
Takes the argument as a string and converts it to uppercase.

```liquid
{{$svc.Metadata.GetValue "foo" | toUpper}}
```

See Go's [strings.ToUpper()](http://golang.org/pkg/strings/#ToUpper) for more information.

##### `contains`

Alias for strings.Contains 

See Go's [strings.Contains()](http://golang.org/pkg/strings/#Contains) for more information.

##### `replace`

Alias for strings.Replace

```liquid
{{$foo := $svc.Labels.GetValue "foo"}}
foo: {{replace $foo "-" "_" -1}}
```

See Go's [strings.Replace()](http://golang.org/pkg/strings/#Replace) for more information.


Examples
--------

TODO
