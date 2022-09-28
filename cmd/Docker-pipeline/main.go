package main

import (
	"flag"
	"log"
	"os"

	"gopkg.in/yaml.v3"

	"scm.atomic-reader.com/Docker-pipeline/internal/pipeline"
)

var input string

func init() {
	flag.StringVar(&input, "manifest", "pipeline.yaml", "The manifest that describes the pipeline tasks")
}

func main() {
	flag.Parse()

	var err error

	var file []byte
	if file, err = os.ReadFile(input); err != nil {
		log.Fatalf("failed to read manifest file: %v", err)
	}

	var manifest pipeline.Manifest
	if err = yaml.Unmarshal(file, &manifest); err != nil {
		log.Fatalf("failed to parse manifest file: %v", err)
	}

	pipeline := pipeline.NewPipeline(manifest)
	if err = pipeline.Execute(flag.Args()); err != nil {
		log.Fatalf("failed to execute the pipeline: %v", err)
	}
}
