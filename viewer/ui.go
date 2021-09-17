package viewer

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
	"strconv"
	"strings"
	"time"
)

var (
	Program          *tea.Program
	Ascii            bool
	formatModeOffset int
)

func getOffsetForLineNumber(a int) int {
	return formatModeOffset - len(strconv.Itoa(a))
}

// selectOption does just that
func selectOption(m *TuiModel) {
	if m.renderSelection || m.helpDisplay {
		return
	}

	m.renderSelection = true
	raw, _, col := m.GetSelectedOption()
	l := len(col)
	row := m.viewport.YOffset + m.mouseEvent.Y - headerHeight

	if row <= l && l > 0 &&
		m.mouseEvent.Y >= headerHeight &&
		m.mouseEvent.Y < m.viewport.Height+headerHeight &&
		m.mouseEvent.X < m.CellWidth()*(len(m.TableHeadersSlice)) {
		if conv, ok := (*raw).(string); ok {
			if format, err := formatJson(conv); err == nil {
				m.selectionText = format
			} else {
				m.selectionText = TruncateIfApplicable(m, conv)
			}
		} else {
			m.selectionText = ""
		}
	} else {
		m.renderSelection = false
	}
}

func swapTableValues(m *TuiModel, f, t *TableState) {
	from := &f.Data
	to := &t.Data
	for k, v := range *from {
		if copyValues, ok := v.(map[string][]interface{}); ok {
			columnNames := m.TableHeaders[k]
			columnValues := make(map[string][]interface{})
			// golang wizardry
			columns := make([]interface{}, len(columnNames))

			for i := range columns {
				columns[i] = copyValues[columnNames[i]][0]
			}

			for i, colName := range columnNames {
				columnValues[colName] = columns[i].([]interface{})
			}

			(*to)[k] = columnValues // data for schema, organized by column
		}
	}
}

func toggleColumn(m *TuiModel) {
	if m.expandColumn > -1 {
		m.expandColumn = -1
	} else {
		m.expandColumn = m.GetColumn()
	}
}

// scrollDown is a simple function to move the viewport down
func scrollDown(m *TuiModel) {
	if m.formatModeEnabled && m.CanFormatScroll {
		m.viewport.YOffset++
		return
	}

	max := getScrollDownMaxForSelection(m)

	if m.viewport.YOffset < max-m.viewport.Height {
		m.viewport.YOffset++
		m.mouseEvent.Y = Min(m.mouseEvent.Y, m.viewport.YOffset)
	}

	if !m.renderSelection {
		m.preScrollYPosition = m.mouseEvent.Y
		m.preScrollYOffset = m.viewport.YOffset
	}
}

// scrollUp is a simple function to move the viewport up
func scrollUp(m *TuiModel) {
	if m.formatModeEnabled && m.CanFormatScroll && m.viewport.YOffset > 0 {
		m.viewport.YOffset--
		return
	}

	if m.viewport.YOffset > 0 {
		m.viewport.YOffset--
		m.mouseEvent.Y = Min(m.mouseEvent.Y, m.viewport.YOffset)
	} else {
		m.mouseEvent.Y = headerHeight
	}

	if !m.renderSelection {
		m.preScrollYPosition = m.mouseEvent.Y
		m.preScrollYOffset = m.viewport.YOffset
	}
}

// TABLE STUFF

// displayTable does some fancy stuff to get a table rendered in text
func displayTable(m *TuiModel) string {
	var (
		builder []string
	)

	// go through all columns
	for c, columnName := range m.TableHeadersSlice {
		if m.expandColumn > -1 && m.expandColumn != c {
			continue
		}

		var (
			rowBuilder []string
		)

		columnValues := m.DataSlices[columnName]
		for r, val := range columnValues {
			base := m.GetBaseStyle().
				UnsetBorderLeft().
				UnsetBorderStyle().
				UnsetBorderForeground()
			s := GetStringRepresentationOfInterface(val)
			s = " " + s
			// handle highlighting
			if c == m.GetColumn() && r == m.GetRow() {
				if !Ascii {
					base.Foreground(lipgloss.Color(highlight()))
				} else if Ascii {
					s = "|" + s
				}
			}
			// display text based on type
			rowBuilder = append(rowBuilder, base.Render(TruncateIfApplicable(m, s)))
		}

		for len(rowBuilder) < m.viewport.Height { // fix spacing issues
			rowBuilder = append(rowBuilder, "")
		}

		column := lipgloss.JoinVertical(lipgloss.Left, rowBuilder...)
		// get a list of columns
		builder = append(builder, m.GetBaseStyle().Render(column))
	}

	// join them into rows
	return lipgloss.JoinHorizontal(lipgloss.Left, builder...)
}

func getFormattedTextBuffer(m *TuiModel) []string {
	margins := headerHeight - footerHeight
	offsetMax := m.viewport.Height - margins
	v := m.selectionText
	validJson := false
	if format, err := formatJson(v); err == nil {
		v = format
		validJson = true
	}

	lines := SplitLines(v)
	formatModeOffset = len(strconv.Itoa(len(lines))) + 1 // number of characters in the numeric string
	lineLength := len(lines)

	ret := []string{}
	m.Format.RunningOffsets = make([]int, lineLength)
	m.Format.NewlineCount = make([]int, lineLength)

	total := 0
	strlen := 0
	for i, v := range lines {
		xOffset := len(strconv.Itoa(i))
		totalOffset := Max(formatModeOffset-xOffset, 0)

		wrap := wordwrap.String(v, m.viewport.Width-totalOffset)
		right := Indent(
			wrap,
			fmt.Sprintf("%d%s", i+m.viewport.YOffset, strings.Repeat(" ", totalOffset)),
			false)
		ret = append(ret, right)
		m.Format.RunningOffsets[i] = total
		m.Format.NewlineCount[i] = strings.Count(wrap, "\n")

		if validJson {
			strlen = len(strings.TrimSpace(v))
		} else {
			strlen = len(v)
		}

		total += strlen
		if strlen > 1 {
			total--
		}
	}
	for i := len(ret); i < offsetMax; i++ {
		ret = append(ret, "")
	}

	return ret
}

func displayFormatBuffer(m *TuiModel) string {
	cpy := make([]string, len(m.Format.Slices))
	for i, v := range m.Format.Slices {
		cpy[i] = *v
	}
	newY := ""
	line := &cpy[m.Format.CursorY]
	x := 0
	offset := getOffsetForLineNumber(m.Format.CursorY)
	for _, r := range *line {
		newY += string(r)
		if x == m.Format.CursorX+offset {
			x++
			break
		}
		x++
	}

	*line += " " // space at the end

	highlight := string((*line)[x])
	newY += lipgloss.NewStyle().Background(lipgloss.Color("#ffffff")).Render(highlight)
	newY += (*line)[x+1:]
	*line = newY
	total := 0
	for i := 0; i < m.viewport.Height; i++ {
		newlines := m.Format.NewlineCount[i+m.viewport.YOffset]
		total += newlines
		if total > m.viewport.Height-headerHeight-footerHeight {
			splits := strings.SplitAfterN(cpy[i], "\n", newlines-1)
			// TODO start here tomorrow
			cpy[i] = splits[0]
			break
		}
	}
	ret := strings.Join(
		cpy,
		"\n")

	return ret
}

// displaySelection does that or writes it to a file if the selection is over a limit
func displaySelection(m *TuiModel) string {
	col := m.GetColumnData()
	m.expandColumn = m.GetColumn()
	row := m.GetRow()
	if m.mouseEvent.Y >= m.viewport.Height+headerHeight && !m.renderSelection { // this is for when the selection is outside the bounds
		return displayTable(m)
	}

	base := m.GetBaseStyle()

	if m.selectionText != "" { // this is basically just if its a string follow these rules
		_, err := formatJson(m.selectionText)
		rows := SplitLines(m.selectionText)
		if err == nil && strings.Contains(m.selectionText, "{") {
			rows = rows[m.viewport.YOffset : m.viewport.Height+m.viewport.YOffset]
		}

		for len(rows) < m.viewport.Height {
			rows = append(rows, "")
		}
		return base.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
	}

	var prettyPrint string
	raw := col[row]

	if conv, ok := raw.(int64); ok {
		prettyPrint = strconv.Itoa(int(conv))
	} else if i, ok := raw.(float64); ok {
		prettyPrint = base.Render(fmt.Sprintf("%.2f", i))
	} else if t, ok := raw.(time.Time); ok {
		str := t.String()
		prettyPrint = base.Render(TruncateIfApplicable(m, str))
	} else if raw == nil {
		prettyPrint = base.Render("NULL")
	}
	if lipgloss.Width(prettyPrint) > maximumRendererCharacters {
		fileName, err := WriteTextFile(m, prettyPrint)
		if err != nil {
			fmt.Printf("ERROR: could not write file %s", fileName)
		}
		return fmt.Sprintf("Selected string exceeds maximum limit of %d characters. \n"+
			"The file was written to your current working "+
			"directory for your convenience with the filename \n%s.", maximumRendererCharacters, fileName)
	}

	lines := SplitLines(prettyPrint)
	for len(lines) < m.viewport.Height {
		lines = append(lines, "")
	}

	prettyPrint = base.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))

	return prettyPrint
}
