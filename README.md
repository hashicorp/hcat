*This package is unreleased, alpha quality that will have API breaking changes
as we get it in shape. We'll do an official release when it is ready.*

# Hashicorp Configuration And Templating (hashicat) library

[![Project unmaintained](https://img.shields.io/badge/project-unmaintained-red.svg)](https://github.com/hashicorp/hcat/issues/132)
[![Go Reference](https://pkg.go.dev/badge/github.com/hashicorp/hcat.svg)](https://pkg.go.dev/github.com/hashicorp/hcat)
[![ci](https://github.com/hashicorp/hcat/actions/workflows/ci.yml/badge.svg)](https://github.com/hashicorp/hcat/actions/workflows/ci.yml)

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

## Diagrams

While the primary documentation for Hashicat is intended to use official godocs
formatting, I thought a few diagrams might help get some aspects across better
and have been working on a few. I'm not great at it but with mermaid I'm hoping
to incrementally improve them over time. Please feel free to file issues/PRs
against them if you have ideas. Thanks.

### Overview

These are some general attempts to get an high level view of what's going on with mixed results. Might be useful...

This diagram is kind of "thing" (struct) oriented. Showing the main structs and
the contact points between them.

``` mermaid
graph TB
    Watcher((Watcher))
    View[View]
    Template[Template]
    TemplateFunction[Template Function]
    Tracker[Tracker]
    Resolver[Resolver]
    Event[Event Notifier]
    Dependency[Dependency]
    Consul{Consul}
    Vault{Vault}

    Watcher --> Template
    Watcher --> Resolver
    Resolver --> Template
    Template --> TemplateFunction
    TemplateFunction --> Dependency
    Template --> Watcher
    Watcher --> View
    View --> Dependency
    Watcher --> Event
    Watcher --> Tracker
    Tracker --> View
    Dependency --> Vault
    Dependency --> Consul
```

This diagram was another attempt at the above but including more information on
what the contact points are and the general flow of things. In it the squares are
structs and the ovals are calls/things-happening.

``` mermaid
flowchart TB
    NW([NewWatcher])
    W[Watcher]
    T[Templates]
    R([Register])
    TN[TrackedNotifers]
    TE([TemplatesEvaluated])
    TF[TemplateFunctions]
    D[Dependencies]
    Rc([Recaller])
    TD[TrackedDependencies]
    V[View]

    NW --> W
    T --> R --> W --> TN
    W --> TE --> TF
    TF --> D--> Rc
    D --> TD
    W --> Rc
    Rc --> V
    V --> W
    TD --- TN

```

### Channels

This shows the main internal channels.

``` mermaid
flowchart TB
    W[Watcher]
    V[View]
    Ti[Timer]

    V -. err-from-dependencies .-> W
    V -.data-from-dependencies.-> W
    Ti -.buffer-period.-> W
    W -.internal-stop.-> W
```

### States

I thought a state diagram was a good idea until I realized there just aren't
that many states.

``` mermaid
stateDiagram-v2
    [*] --> Initialized
    Initialized --> NotifiersTracked: templates registered
    NotifiersTracked --> ResovingDependencies: templates run
    ResovingDependencies --> ResovingDependencies: templates run
    ResovingDependencies --> Watching: steady state achieved
    Watching --> ResovingDependencies: data updates
    Watching --> [*]: stop
```

### Template.Execute() Flow

This is probably one of the more useful diagrams, dipicting the call flow of
a Template execution. Note that "Dirty" is a term I swiped from filesystems, it
denotes that some data that the template uses has been changed.

``` mermaid
flowchart TB
    Start --> Execute
    Execute --> D{Dirty?}
    D -->|no| Rc[Return Cache]
    D -->|yes| TE[Template Exec]
    TE --> TF[Template Functions]
    TF --> R[Recaller]
    R --> Tr[Tracker]
    R --> Ca{Cache?}
    Ca -->|hit|Rd[Return Data]
    Ca -->|miss| Poll
    Poll --> Dep[Dependency]
    Dep --> Cl((Cloud))
    Cl --> Dep
    Dep --> Poll
    Poll --> Ca
```

### Watcher.Wait() Flow

Similar to the above.. What happens when you call watcher.Wait()?

``` mermaid
flowchart TB
    Start --> Wait
    Wait --> S{Select?}
    S -->|dataChan| NewData
    S -->|bufferTimer| Return
    S -->|stopChan| Return
    S -->|errChan| Return
    S -->|context.Done| Return
    NewData --> SC[Save To Cache]
    NewData --> N{Notifier approved?}
    N -->|yes| B{Buffering?}
    B -->|yes| S
    B -->|no| Return
    N -->|no| Return
```

