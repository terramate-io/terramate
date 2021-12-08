package hcl

import (
	"io"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

type Printer struct{}

func (Printer) PrintTerramate(w io.Writer, ts Terramate) error {
	f := hclwrite.NewEmptyFile()
	rootBody := f.Body()
	tsBlock := rootBody.AppendNewBlock("terramate", nil)
	tsBody := tsBlock.Body()
	tsBody.SetAttributeValue("required_version", cty.StringVal(ts.RequiredVersion))
	_, err := w.Write(f.Bytes())
	return err
}
