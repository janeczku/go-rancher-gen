go-rancher-gen
===============
[![Latest Version](https://img.shields.io/github/release/janeczku/go-rancher-gen.svg?maxAge=600)][release]
[![CircleCI](https://img.shields.io/circleci/project/janeczku/go-rancher-gen.svg)][circleci]
[![Docker Pulls](https://img.shields.io/docker/pulls/janeczku/rancher-gen.svg?maxAge=600)][hub]
[![License](https://img.shields.io/github/license/janeczku/go-rancher-gen.svg?maxAge=600)]()

[release]: https://github.com/janeczku/go-rancher-gen/releases
[circleci]: https://circleci.com/gh/janeczku/go-rancher-gen
[hub]: https://hub.docker.com/r/janeczku/go-rancher-gen/

`rancher-gen` is a file generator that renders templates using [Rancher Metadata](http://docs.rancher.com/rancher/metadata-service/).

**Core features:**

+ Powerful template syntax that embraces [Rancher](http://www.rancher.com) services, containers and hosts as first-class objects
+ Ability to run arbitrary commands when a file has been updated (e.g. to reload an application's configuration)
+ Ability to run check commands on staged files before updating the destination files
+ Ability to specify multiple template sets using a TOML config file

Usage
------------

### Command Line

``` rancher-gen [options] source [dest]```

#### options

|       Flag         |            Description         |
| ------------------ | ------------------------------ |
| `config`           | Path to an optional config file. Options specified on the CLI take precedence over those in the config file.
| `metadata-version` | Metadata version string used when querying the Rancher Metadata API. Default: `latest`.
| `include-inactive` | *Not yet implemented*
| `interval`         | Interval (in seconds) for polling the Metadata API for changes. Default: `5`
| `onetime`          | Process all templates once and exit. Default: `false`
| `log-level`        | Verbosity of log output. Valid values: "debug", "info", "warn", and "error". Default: `info`.
| `check-cmd`        | Command to check the content before updating the destination file. Use the `{{staging}}` placeholder to reference the staging file.
| `notify-cmd`       | Command to run after the destination file has been updated.
| `notify-output`    | Print the result of the notify command to STDOUT.
| `version`          | Show application version and exit.

#### source
Path to the template.

#### dest
Path to the destination file. If omitted, then the generated content is printed to STDOUT.

### Examples

```
rancher-gen --onetime --notify-cmd="/usr/sbin/service nginx reload" \
/etc/rancher-gen/nginx.tmpl /etc/nginx/nginx.conf
```

```
rancher-gen --interval 2 --check-cmd="/usr/sbin/nginx -t -c {{staging}}" \
--notify-cmd="/usr/sbin/service nginx reload" /etc/rancher-gen/nginx.tmpl /etc/nginx/nginx.conf
```

### Configuration file

You can optionally pass a configuration file to `rancher-gen`. The configuration file is a [TOML](https://github.com/toml-lang/toml) file. It allows you to specify multiple template sets grouped by `template` sections. You can specify the same options as on the command line. Options specified on the command line or via environment variables take precedence over the corresponding values in the configuration file. An example file is available [here](examples/config.toml.sample).

How to dynamically configure your applications with Rancher Metadata
------------

You can bundle `rancher-gen` with the application image or run it as a [service sidekick](http://docs.rancher.com/rancher/rancher-compose/#sidekicks), that exposes the generated configuration file in a shared volume.

### Bundled with application image

Download the binary from the [release page][release].
Add the binary to your Docker image and provide a mechanism that runs `rancher-gen` on container start and then executes the main application. This functionality could be provided by a Bash script executed as image `ENTRYPOINT`. If you want to reload the application whenever the Metadata referenced in the template changes, you can use a container process supervisor (e.g. [S6-overlay](https://github.com/just-containers/s6-overlay)) to keep `rancher-gen` running in the background and notify the application when it needs to reload the configuration (by sending it a SIGHUP for example).

### Sidekick Container
Create a new Docker image using `janeczku/rancher-gen:latest` as base. Add the template(s) and configuration file(s) to the image. Expose the configuration folder as `VOLUME`.
Run `rancher-gen` on container start, specifying relevant options as command line parameters.

##### Example acme/nginx-config sidekick image

```DOCKERFILE
FROM janeczku/rancher-gen:latest
COPY config.toml /etc/rancher-gen/
COPY nginx.tmpl /etc/rancher-gen/
VOLUME /etc/nginx
CMD ["--config", "/etc/rancher-gen/config.toml"]
```

##### Example Rancher Compose file

```YAML
nginx:
  image: nginx:latest
  volumes_from:
  - config-sidekick
  labels:
    io.rancher.sidekicks: template-sidekick
config-sidekick:
  image: acme/nginx-config
```

Template Language
------------
Templates are [Go text templates](http://golang.org/pkg/text/template/).
In addition to the built-in functions, `rancher-gen` exposes functions and methods to easily discover Rancher services, containers and hosts.

### Service Discovery Objects

```go
type Service struct {
	Name        string
	Stack       string
	Kind        string
	Vip         string
	Fqdn        string
	Ports       []Port
	Labels      LabelMap
	Metadata    MetadataMap
	Containers  []Container
}

type Port struct {
	PublicPort   string
	InternalPort string
	Protocol     string
}

type Container struct {
	Name        string
	Address     string
	Stack       string
	Service     string
	Health      string
	State       string
	Labels      LabelMap
	Host        Host
}

type Host struct {
	UUID        string
	Name        string
	Address     string
	Hostname    string
	Labels      LabelMap
}
```

The `LabelMap` and `MetadataMap` types implement methods for easily checking the existence of specific keys and accessing their values:

**`Labels.Exists(key string) bool`**    
Returns true if the given label key exists in the map.

**`Labels.GetValue(key, default string) string`**    
Returns the value of the given label key. The function accepts an optional default value that is returned when the key doesn't exist or is set to an empty string.

**`Metadata.Exists(key string) bool`**   
Returns true if the given metadata key exists in the map.

**`Metadata.GetValue(key, default interface{}) interface{}`**    
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
{{with service "web.production"}}
{{range .Containers}}
http://{{.Address}}:9090
{{end}}
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
{{$ervice := service "web.production"}}
{{range $container := whereLabelEquals "foo" "bar" $service.Containers}}
{{do something with $container}}
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

#### `base`

Alias for the path.Base function

```liquid
filename: {{$service.Metadata.GetValue "targetPath" | base}}
```

See Go's [path.Base()](https://golang.org/pkg/path/#Base) for more information.

#### `dir`

Alias for the path.Dir function

See Go's [path.Dir()](https://golang.org/pkg/path/#Dir) for more information.

#### `env`

Returns the value of the given environment variable or an empty string if the variable isn't set

```liquid
{{env "FOO_VAR"}}
```

#### `timestamp`

Alias for time.Now

```liquid
# Generated by rancher-gen {{timestamp}}
```

The timestamp can be formatted as required by invoking the `Format` method:

```liquid
# Generated by rancher-gen {{timestamp.Format "Jan 2, 2006 15:04"}}
```

See Go's [time.Format()](http://golang.org/pkg/time/#Time.Format) for more information about formatting the date according to the layout of the reference time.

#### `split`

Alias for strings.Split

```liquid
{{$items := split $someString ":"}}
```

See Go's [strings.Split()](http://golang.org/pkg/strings/#Split) for more information.

#### `join`

Alias for strings.Join    
Takes the given slice of strings as a pipe and joins them on the provided string:

```liquid
{{$items | join ","}}
```

See Go's [strings.Join()](http://golang.org/pkg/strings/#Join) for more information.

#### `toLower`

Alias for strings.ToLower    
Takes the argument as a string and converts it to lowercase.

```liquid
{{$svc.Metadata.GetValue "foo" | toLower}}
```

See Go's [strings.ToLower()](http://golang.org/pkg/strings/#ToLower) for more information.

#### `toUpper`

Alias for strings.ToUpper    
Takes the argument as a string and converts it to uppercase.

```liquid
{{$svc.Metadata.GetValue "foo" | toUpper}}
```

See Go's [strings.ToUpper()](http://golang.org/pkg/strings/#ToUpper) for more information.

#### `contains`

Alias for strings.Contains 

See Go's [strings.Contains()](http://golang.org/pkg/strings/#Contains) for more information.

#### `replace`

Alias for strings.Replace

```liquid
{{$foo := $svc.Labels.GetValue "foo"}}
foo: {{replace $foo "-" "_" -1}}
```

See Go's [strings.Replace()](http://golang.org/pkg/strings/#Replace) for more information.


Examples
--------

TODO
