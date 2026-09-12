package main

import (
	"github.com/coalaura/plain"
)

var log = plain.New(plain.WithDate(plain.RFC3339Local))

func main() {
	log.Println("Reading workflows...")

	workflows, err := ReadWorkflows()
	log.MustFail(err)

	log.Println("Fetching latest releases...")

	results, err := FetchLatestReleases(workflows, false)
	log.MustFail(err)

	log.Println("Updating workflows...")

	for _, workflow := range workflows {
		err = workflow.Update(results)
		log.MustFail(err)
	}

	log.Println("Done.")
}
