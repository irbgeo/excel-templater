package main

import (
	_ "embed"
	"encoding/json"

	"github.com/xuri/excelize/v2"

	"github.com/geoirb/excel-templater"
)

var (
	templateFile = "./examples/template.xlsx"
	resultFile   = "./examples/result.xlsx"
	useDefault   = false

	//go:embed payload.json
	data []byte
)

func main() {
	templater := excel.NewTemplater(useDefault)

	var payload interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		panic(err)
	}

	r, err := templater.FillIn(templateFile, payload)
	if err != nil {
		panic(err)
	}
	file, err := excelize.OpenReader(r)
	if err != nil {
		panic(err)
	}

	if err = file.SaveAs(resultFile); err != nil {
		panic(err)
	}
}
