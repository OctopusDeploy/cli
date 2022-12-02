package output

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/muesli/reflow/ansi"
	"golang.org/x/term"
)

const (
	delimiter     = "  "
	delimiterSize = len(delimiter)
	defaultWidth  = 99999 // when running inside a script or CI system or something, we never want to truncate table output
)

type DataRow struct {
	Name  string
	Value string
}

func NewDataRow(name string, value string) *DataRow {
	return &DataRow{
		Name:  name,
		Value: value,
	}
}

type Table interface {
	AddRow(...string)
	Print() error
}

type table struct {
	out      io.Writer
	maxWidth int
	rows     [][]string
}

func PrintRows(rows []*DataRow, w io.Writer) {
	t := NewTable(w)
	for _, row := range rows {
		t.AddRow(row.Name, row.Value)
	}
	t.Print()
}

func NewTable(writer io.Writer) Table {
	width := defaultWidth
	if term.IsTerminal(int(os.Stdin.Fd())) {
		width, _, _ = term.GetSize(int(os.Stdout.Fd()))
		if width < 1 {
			width = defaultWidth
		}
	}

	return &table{
		out:      writer,
		maxWidth: width,
	}
}

func (t *table) AddRow(s ...string) {
	t.rows = append(t.rows, s)
}

func (t *table) Print() error {
	if len(t.rows) == 0 {
		return nil
	}
	colLen := len(t.rows[0])
	colWidths := t.calcColWidths()

	for _, row := range t.rows {
		for col, field := range row {
			if col > 0 {
				_, err := fmt.Fprint(t.out, delimiter)
				if err != nil {
					return err
				}
			}
			truncVal := Truncate(colWidths[col], field)
			if col < colLen-1 {
				if padWidth := colWidths[col] - ansi.PrintableRuneWidth(field); padWidth > 0 {
					truncVal += strings.Repeat(" ", padWidth)
				}
			}
			_, err := fmt.Fprint(t.out, truncVal)
			if err != nil {
				return err
			}
		}
		if len(row) > 0 {
			_, err := fmt.Fprint(t.out, "\n")
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *table) calcColWidths() []int {
	colLen := len(t.rows[0])
	allColWidths := make([][]int, colLen)
	for _, row := range t.rows {
		for col, field := range row {
			allColWidths[col] = append(allColWidths[col], ansi.PrintableRuneWidth(field))
		}
	}

	maxColWidths := make([]int, colLen)
	for col, widths := range allColWidths {
		sort.Ints(widths)
		maxColWidths[col] = widths[len(widths)-1]
	}

	colWidths := make([]int, colLen)
	// don't truncate the first col
	colWidths[0] = maxColWidths[0]

	// don't truncate last column if it displays URLs
	if strings.HasPrefix(t.rows[0][colLen-1], "https://") {
		colWidths[colLen-1] = maxColWidths[colLen-1]
	}

	availWidth := func() int {
		setWidths := 0
		for col := 0; col < colLen; col++ {
			setWidths += colWidths[col]
		}
		return t.maxWidth - delimiterSize*(colLen-1) - setWidths
	}
	numFixedCols := func() int {
		fixedCols := 0
		for col := 0; col < colLen; col++ {
			if colWidths[col] > 0 {
				fixedCols++
			}
		}
		return fixedCols
	}

	// set the widths of short columns
	if w := availWidth(); w > 0 {
		if numFlexColumns := colLen - numFixedCols(); numFlexColumns > 0 {
			perColumn := w / numFlexColumns
			for col := 0; col < colLen; col++ {
				if max := maxColWidths[col]; max < perColumn {
					colWidths[col] = max
				}
			}
		}
	}

	firstFlexCol := -1
	// truncate long columns to the remaining available width
	if numFlexColumns := colLen - numFixedCols(); numFlexColumns > 0 {
		perColumn := availWidth() / numFlexColumns
		for col := 0; col < colLen; col++ {
			if colWidths[col] == 0 {
				if firstFlexCol == -1 {
					firstFlexCol = col
				}
				if max := maxColWidths[col]; max < perColumn {
					colWidths[col] = max
				} else if perColumn > 0 {
					colWidths[col] = perColumn
				}
			}
		}
	}

	// add remainder to the first flex column
	if w := availWidth(); w > 0 && firstFlexCol > -1 {
		colWidths[firstFlexCol] += w
		if max := maxColWidths[firstFlexCol]; max < colWidths[firstFlexCol] {
			colWidths[firstFlexCol] = max
		}
	}

	return colWidths
}
