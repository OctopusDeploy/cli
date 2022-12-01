package shared

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
