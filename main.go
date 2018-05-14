//go:generate gorunpkg github.com/vektah/gqlgen -typemap graph/types.json -out graph/generated.go -package graph

package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/nycmonkey/gqlftw/graph"
	"github.com/vektah/gqlgen/handler"
)

func main() {
	app, err := graph.New()
	if err != nil {
		log.Fatalln(err)
	}
	http.Handle("/", handler.Playground("Todo", "/query"))
	http.Handle("/query", handler.GraphQL(graph.MakeExecutableSchema(app)))

	fmt.Println("Listening on :8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}
