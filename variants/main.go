package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/bertinatto/testgrid/internal"
)

func main() {
	input := flag.String("input", "", "input TSV file")
	output := flag.String("output", "", "output file")
	flag.Parse()

	if *input == "" {
		fmt.Fprintf(os.Stderr, "Input file is required\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if *output == "" {
		fmt.Fprintf(os.Stderr, "Output file is required\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	data, err := readTSVFile(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read TSF file %s: %v\n", *input, err)
		os.Exit(1)
	}

	err = generateGoFile(*output, data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate .go file %s: %v\n", *output, err)
		os.Exit(1)
	}

	fmt.Printf("Go file generated: %s\n", *output)
}

func readTSVFile(filename string) (map[string]internal.Variant, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data := make(map[string]internal.Variant, 128)
	reader := csv.NewReader(file)
	reader.Comma = '\t'

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) < 1 {
		return nil, fmt.Errorf("invalid TSV file: not enough records")
	}

	// Start from index 1 to discard headers
	for i := 1; i < len(records); i++ {
		line := records[i]
		job := line[0]
		variants := line[1]
		extendedVariants := line[2]

		extVarSplit := strings.Split(extendedVariants, ",")
		v := internal.Variant{
			Name:                variants,
			Parallel:            contains(extVarSplit, "parallel"),
			CSI:                 contains(extVarSplit, "csi"),
			UpgradeFromCurrent:  contains(extVarSplit, "upgrade"),
			UpgradeFromPrevious: contains(extVarSplit, "upgrade-minor"),
			Serial:              contains(extVarSplit, "serial"),
		}

		data[job] = v
	}

	return data, nil
}

func contains(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

func generateGoFile(filename string, data map[string]internal.Variant) error {
	defer func() {
		if _, err := exec.Command("gofmt", "-s", "-w", filename).Output(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v", err)
			os.Exit(1)
		}
	}()

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	header := `
package generated

import	"github.com/bertinatto/testgrid/internal"

// This file is generated by go generate. DO NOT EDIT.

var Variants = map[string]internal.Variant{
`

	footer := `
}
`
	entryFmt := `
"%s": {
	Name: "%s",
	Parallel: %v,
	CSI: %v,
	UpgradeFromPrevious: %v,
	UpgradeFromCurrent: %v,
	Serial: %v,
},`
	_, err = file.WriteString(header)
	if err != nil {
		return err
	}

	for _, job := range sortedKeys(data) {
		v := data[job]
		line := fmt.Sprintf(entryFmt, job, v.Name, v.Parallel, v.CSI, v.UpgradeFromPrevious, v.UpgradeFromCurrent, v.Serial)
		_, err := file.WriteString(line)
		if err != nil {
			return err
		}
	}

	_, err = file.WriteString(footer)

	return err
}

func sortedKeys(data map[string]internal.Variant) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
