# GQLFTW
Fun with [GraphQL](httpd://graphql.org/), [SPARQL](https://en.wikipedia.org/wiki/SPARQL) and [Wikidata](https://query.wikidata.org/)

## Tools
1. The [Go](https://golang.org) programming language
2. [dep](https://github.com/golang/dep) for dependency management in Go
3. [gqlgen](https://github.com/vektah/gqlgen), a code generation tool to build strongly typed GraphQL servers in Go
4. [dataloaden](https://github.com/vektah/dataloaden) for generating data laoder functions
5. [Wikidata Query Service](https://query.wikidata.org/) to query Wikipedia data with SPARQL

## Setup to recreate this project
1. Install Go and set the GOPATH environment variable to a directory containing three folders: `src`, `pkg` and `bin`
2. Download a [release]() of [dep](https://github.com/golang/dep) to your PATH
3. Create a project folder (common convention is to use `$GOPATH/github.com/:username/:projectname`)
4. Install [gqlgen](https://gqlgen.com/getting-started/) with `go get github.com/vektah/gqlgen`
5. Install [gorunpkg](https://github.com/vektah/gorunpkg) with `go get github.com/vektah/gorunpkg`. That tool will use vendored versions of the code generation tools to ensure that dependencies stay locked to predictable versions.
6. Install [dataloaden](https://github.com/vektah/dataloaden) with `go get github.com/vektah/dataloaden`

The steps below are modified from [this excellent guide](https://gqlgen.com/getting-started/).

## Concepts

`gqlgen` inspects a GraphQL schema and corresponding Go code to generate most of the plumbing required to expose data via GraphQL.  Types from the GraphQL schema are mapped to Go types via a mapping file.  The mapping file is just json mapping a string GraphQL type to a fully qualified Go type.  For example, the following mapping indicates that the GraphQL type `Company` maps to the Go type `Company` in the package `github.com/nycmonkey/gqlftw/model`.
```
{
    "Company": "github.com/nycmonkey/gqlftw/model.Company"
}
```

`gqlgen` will inspect the schema and corresponding model files.  Any public struct fields or functions on the Go type that match the names on the GraphQL type (without regard to case) will be mapped automatically.  For everything else, `gqlgen` will define an interface with the function signatures that must be satisfied by your resolver.  It is up to you to determine how to load the data; anything that satisfies the interface is fine.

## First implementation steps 

1. Create a project folder for your code under `$GOPATH/src`.  A common go idiom is to use `github.com/:username/:projectname`.  We'll refer to that as the "project root"
2. Create folders under the project root:
    1. graph
    2. model
2. Create a `main.go` file in the project root with the minimum content to compile:
    ```
    //go:generate gorunpkg github.com/vektah/gqlgen -typemap graph/types.json -out graph/generated.go -package graph
    
    package main
    import (
    	"fmt"
    	"log"
    	"net/http"

    	"{{YOUR_PROJECT_ROOT_HERE}}/graph"
    	"github.com/vektah/gqlgen/handler"
    )

    func main() {
    	app := &graph.MyApp{}
    	http.Handle("/", handler.Playground("Todo", "/query"))
    	http.Handle("/query", handler.GraphQL(graph.MakeExecutableSchema(app)))

    	fmt.Println("Listening on :8080")
    	log.Fatal(http.ListenAndServe(":8080", nil))
    }
    ```
3. Write a [graphql schema](https://graphql.org/learn/schema/) and save it in the project root as `schema.graphql`.  Focus on the data and its connections, not the server implementation.
4. Write the model objects (structs) that will hold the data in memory. Remember, if the type exposes properties or methods that match the names of the corresponding GraphQL type, you won't need to write a resolver function.
5. Map your GraphQL types to Go types in `graph/types.json` (see the Concepts section above for an example)
6. Generate the plumbing code by typing `go generate ./...` from the project root.  The comment at the top of the `main.go` file will be read by that command.  It should call `gqlgen` with the necessary arguments. If your folder structure varies from the layout suggested here, you may need to modify the flags
7. Try to build the project with `go build`.  You should see a message that `graph.MyApp` is undefined.
8. Create a new file `graph/graph.go`:
    ```
    // graph/graph.go
    package graph

    import (
        "context"
        "fmt"
        "math/rand"
    )

    type MyApp struct {}
    ```
Implement the methods of the `Resolvers` interface in `graph/generated.go` on MyApp, then try to compile the server with `go build`
9. Vendor the project dependencies with `dep init` and `dep ensure`
10. At this point, you should have a working graphql service.  If you execute the binary generated from `go build` or run `go run main.go`, you can play with a graphql playground at http://localhost:8080/ or submit GrapQL queries to http://localhost:8080/query.  However, your requests will suffer from [the N+1 problem](https://secure.phabricator.com/book/phabcontrib/article/n_plus_one/)

## Introducing dataloader

Dataloader sounds intimidating but the surface area is quite small.  When implemented, dataloader functions expose to primary functions: `Load(key) (result, error)` and `LoadAll([]key) ([]result, []error)`.  Behind the scenes, the implementation will wait for up to a designated period of time for up to a max number of requests, and then send the unique set of requested keys to your loader implementation. Your loader implementation will have the signature `Load([]key) ([]result, []error)`.  It is your responsibility to ensure that the implementation returns results __in the same order as the keys are received__, this ensuring that the dataloader implementation can route the responses back to the correct requests.  If there is no result available for a particular key, return a nil value or the zero value of the type (the generated interface will follow whatever you specify in your GraphQL schema).

Dataloaders are typically scoped by request by instantiating a new one per request via a context.  This allows for different users to receive different responses from the same service, without leaking info to the wrong recipient.  of course, there is no such requirement -- you can instantiate global dataloaders if you choose.  Note that the `dataloaden` library suggested here implements a lazily loaded cache with a simple hash table.  If you leave it running in a global context its memory usage will grow for each new key visited.