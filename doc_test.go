package hcat

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// These examples requires a running consul to test against.
// For testing it is taken care of by TestMain.

const (
	exampleServiceTemplate = "{{range services}}{{range service .Name }}" +
		"service {{.Name }} at {{.Address}}" +
		"{{end}}{{end}}"
	exampleNodeTemplate = "{{range nodes}}node at {{.Address}}{{end}}"
)

var examples = []string{exampleServiceTemplate, exampleNodeTemplate}

// Repeatedly runs the resolver on the template and watcher until the returned
// ResolveEvent shows the template has fetched all values and completed, then
// returns the output.
func RenderExampleOnce(addr string) string {
	tmpl := NewTemplate(TemplateInput{
		Contents: exampleServiceTemplate,
	})
	clients := NewClientSet()
	clients.AddConsul(ConsulInput{Address: addr})
	w := NewWatcher(WatcherInput{
		Clients: clients,
		Cache:   NewStore(),
	})

	ctx := context.Background()
	r := NewResolver()
	for {
		re, err := r.Run(tmpl, w)
		if err != nil {
			log.Fatal(err)
		}
		if re.Complete {
			return string(re.Contents)
		}
		// Wait pauses until new data has been received
		err = w.Wait(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}
}

// Runs the resolver over multiple templates until all have completed.
// By looping over all the templates it can start the data lookups in each and
// better share cached results for faster overall template rendering.
func RenderMultipleOnce(addr string) string {
	templates := make([]*Template, len(examples))
	for i, egs := range examples {
		templates[i] = NewTemplate(TemplateInput{Contents: egs})
	}
	clients := NewClientSet()
	clients.AddConsul(ConsulInput{Address: addr})
	w := NewWatcher(WatcherInput{
		Clients: clients,
		Cache:   NewStore(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	results := []string{}
	r := NewResolver()
	for {
		for _, tmpl := range templates {
			re, err := r.Run(tmpl, w)
			if err != nil {
				log.Fatal(err)
			}
			if re.Complete {
				results = append(results, string(re.Contents))
			}
		}
		if len(results) == len(templates) {
			break
		}
		// Wait pauses until new data has been received
		err := w.Wait(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}
	return strings.Join(results, ", ")
}

// Shows multiple examples of usage from a high level perspective.
func Example() {
	if *runExamples {
		// consuladdr is set in TestMain
		fmt.Printf("RenderExampleOnce: %s\nRenderMultipleOnce: %s\n",
			RenderExampleOnce(consuladdr),
			RenderMultipleOnce(consuladdr))
	} else {
		// so test doesn't fail when skipping
		fmt.Printf("RenderExampleOnce: %s\nRenderMultipleOnce: %s\n",
			"service consul at 127.0.0.1",
			"node at 127.0.0.1, service consul at 127.0.0.1")
	}
	// Output:
	// RenderExampleOnce: service consul at 127.0.0.1
	// RenderMultipleOnce: node at 127.0.0.1, service consul at 127.0.0.1
}
