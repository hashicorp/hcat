# Hashicorp Configuration And Templating library

[![Go Documentation](https://img.shields.io/badge/go-documentation-%2300acd7)](https://godoc.org/github.com/hashicorp/hcat)

This library provides a means to fetch data managed by external services and
render templates using that data. It also enables monitoring those services for
data changes to trigger updates to the templates.

It currently supports Consul and Vault as data sources, but we expect to add
more soon.

This library was originally based on the code from Consul-Template with a fair
amount of refactoring.

## Community Support

If you have questions about hashicat, its capabilities or anything other than a
bug or feature request (use github's issue tracker for those), please see our
community support resources.

Community portal: https://discuss.hashicorp.com/c/consul

Other resources: https://www.consul.io/community.html

Additionally, for issues and pull requests we'll be using the :+1: reactions as
a rough voting system to help gauge community priorities. So please add :+1: to
any issue or pull request you'd like to see worked on. Thanks.

## Basic Usage

A simple example of how you might use this library to generate the contents of
a single template, waiting for all its dependencies (external data) to be
fetched and filled in, then have that content returned.

```go
func WaitForTemplate(tmpl *hcat.Template, w *hcat.Watcher)([]byte, error) {
	for {
		re, err := r.Run(tmpl, w)
		if re.Complete || err != nil {
			return re.Contents, err
		}
		err = w.Wait(timeout)
		if err != nil {
			return nil, err
		}
	}
}
```
