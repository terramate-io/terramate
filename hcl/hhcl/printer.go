package hhcl

import (
	"io"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terrastack/hcl"
	"github.com/zclconf/go-cty/cty"
)

type Printer struct{}

func NewPrinter() Printer { return Printer{} }

func (p *Printer) PrintTerrastack(w io.Writer, ts hcl.Terrastack) error {
	f := hclwrite.NewEmptyFile()
	rootBody := f.Body()
	tsBlock := rootBody.AppendNewBlock("terrastack", nil)
	tsBody := tsBlock.Body()
	tsBody.SetAttributeValue("required_version", cty.StringVal(ts.RequiredVersion))
	_, err := w.Write(f.Bytes())
	return err
}
