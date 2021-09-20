package viewer

import (
	tea "github.com/charmbracelet/bubbletea"
)

var (
	inputBlacklist = []string{
		"alt+[",
		"up",
		"down",
		"tab",
		"left",
		"enter",
		"right",
		"pgdown",
		"pgup",
	}
)

func prepareFormatMode(m *TuiModel) {
	m.formatModeEnabled = true
	m.editModeEnabled = false
	m.textInput.Model.SetValue("")
	m.formatInput.Model.SetValue("")
	m.formatInput.Model.focus = true
	m.textInput.Model.focus = false
	m.textInput.Model.Blur()
}

func moveCursorWithinBounds(m *TuiModel) {
	defer func() {
		if recover() != nil {
			println("whoopsy")
		}
	}()
	offset := getOffsetForLineNumber(m.Format.CursorY)
	l := len(*m.Format.Slices[m.Format.CursorY])

	end := l - 1 - offset
	if m.Format.CursorX > end {
		m.Format.CursorX = end
	}
}

func handleEditInput(m *TuiModel, str, val string) (ret bool) {
	selectedInput := &m.textInput.Model
	input := selectedInput.Value()
	inputLen := len(input)

	if str == "backspace" {
		cursor := selectedInput.Cursor()
		runes := []rune(input)
		if cursor == inputLen && inputLen > 0 {
			selectedInput.SetValue(input[0 : inputLen-1])
		} else if cursor > 0 {
			min := Max(selectedInput.Cursor(), 0)
			min = Min(min, inputLen-1)
			first := runes[:min-1]
			last := runes[min:]
			selectedInput.SetValue(string(first) + string(last))
			selectedInput.SetCursor(selectedInput.Cursor() - 1)
		}

		ret = true
	} else if str == "enter" { // writes your selection
		m.textInput.EnterBehavior(m, selectedInput, input)
		ret = true
	}

	return ret
}

func handleEditMovement(m *TuiModel, str, val string) (ret bool) {
	selectedInput := &m.textInput.Model
	if str == "home" {
		selectedInput.setCursor(0)

		ret = true
	} else if str == "end" {
		if len(val) > 0 {
			selectedInput.setCursor(len(val) - 1)
		}

		ret = true
	} else if str == "left" {
		cursorPosition := selectedInput.Cursor()

		if cursorPosition == selectedInput.offset && cursorPosition != 0 {
			selectedInput.offset--
			selectedInput.offsetRight--
		}

		if cursorPosition != 0 {
			selectedInput.SetCursor(cursorPosition - 1)
		}

		ret = true
	} else if str == "right" {
		cursorPosition := selectedInput.Cursor()

		if cursorPosition == selectedInput.offsetRight {
			selectedInput.offset++
			selectedInput.offsetRight++
		}

		selectedInput.setCursor(cursorPosition + 1)

		ret = true
	}

	return ret
}

func handleFormatMovement(m *TuiModel, str string) (ret bool) {
	lines := 0
	for _, v := range m.Format.Slices {
		if *v != "" {
			lines++
		}
	}
	switch str {
	case "pgdown":
		l := len(m.Format.Text) - 1
		for i := 0; i < m.viewport.Height && m.viewport.YOffset < l; i++ {
			scrollDown(m)
		}
		ret = true
		break
	case "pgup":
		for i := 0; i <
			m.viewport.Height && m.viewport.YOffset > 0; i++ {
			scrollUp(m)
		}
		ret = true
		break
	case "home":
		m.viewport.YOffset = 0
		m.Format.CursorX = 0
		m.Format.CursorY = 0
		ret = true
		break
	case "end":
		m.viewport.YOffset = len(m.Format.Text) - m.viewport.Height
		m.Format.CursorY = m.viewport.Height - footerHeight
		m.Format.CursorX = m.Format.RunningOffsets[len(m.Format.RunningOffsets)-1]
		ret = true
		break
	case "right":
		ret = true
		m.Format.CursorX++

		offset := getOffsetForLineNumber(m.Format.CursorY)
		x := m.Format.CursorX + offset + 1 // for the space at the end
		l := len(*m.Format.Slices[m.Format.CursorY])
		maxY := lines - 1
		if l < x && m.Format.CursorY < maxY {
			m.Format.CursorX = 0
			m.Format.CursorY++
		} else if l < x && m.Format.CursorY < len(m.Format.Text)-1 {
			go Program.Send(
				tea.KeyMsg{
					Type: tea.KeyDown,
					Alt:  false,
				},
			)
		} else if m.Format.CursorY > maxY {
			m.Format.CursorX = maxY
		}

		break
	case "left":
		ret = true
		m.Format.CursorX--

		if m.Format.CursorX < 0 && m.Format.CursorY > 0 {
			m.Format.CursorY--

			offset := getOffsetForLineNumber(m.Format.CursorY)
			l := len(*m.Format.Slices[m.Format.CursorY])
			m.Format.CursorX = l - 1 - offset
		} else if m.Format.CursorX < 0 &&
			m.Format.CursorY == 0 &&
			m.viewport.YOffset > 0 {
			go Program.Send(
				tea.KeyMsg{
					Type: tea.KeyUp,
					Alt:  false,
				},
			)
		} else if m.Format.CursorX < 0 {
			m.Format.CursorX = 0
		}

		break
	case "up":
		ret = true
		if m.Format.CursorY > 0 {
			m.Format.CursorY--
		} else if m.viewport.YOffset > 0 {
			scrollUp(m)
		}

		break
	case "down":
		ret = true
		if m.Format.CursorY < m.viewport.Height-footerHeight && m.Format.CursorY < lines-1 {
			m.Format.CursorY++
		} else {
			scrollDown(m)
		}
	}

	return ret
}

func insertCharacter(m *TuiModel, newlineOrTab string) {
	yOffset := Max(m.viewport.YOffset, 0)
	cursor := m.Format.RunningOffsets[m.Format.CursorY+yOffset] + m.Format.CursorX
	runes := []rune(m.selectionText)
	if runes[Min(cursor, len(runes)-1)] == '\n' && newlineOrTab == "\t" {
		return
	}
	min := Max(cursor, 0)
	min = Min(min, len(m.selectionText))
	first := runes[:min]
	last := runes[min:]
	f := string(first)
	l := string(last)
	m.selectionText = f + newlineOrTab + l
	if len(last) == 0 { // for whatever reason, if you don't double up on newlines if appending to end, it gets removed
		m.selectionText += newlineOrTab
	}
	numLines := 0
	for _, v := range m.Format.Text {
		if v != "" { // ignore padding
			numLines++
		}
	}
	if yOffset+m.viewport.Height == numLines && newlineOrTab == "\n" {
		m.viewport.YOffset++
	} else if newlineOrTab == "\n" {
		m.Format.CursorY++
	}

	m.Format.Text = getFormattedTextBuffer(m)
	m.SetViewSlices()
	if newlineOrTab == "\n" {
		m.Format.CursorX = 0
	} else {
		m.Format.CursorX++
	}
}

func handleFormatInput(m *TuiModel, str string) bool {
	switch str {
	case "tab":
		insertCharacter(m, "\t")
		return true
	case "enter":
		insertCharacter(m, "\n")
		return true
	case "backspace":
		cursor := m.Format.CursorX + formatModeOffset
		input := m.Format.Slices[m.Format.CursorY]
		inputLen := len(*input)
		runes := []rune(*input)
		if m.Format.CursorX > 0 {
			if cursor == inputLen && inputLen > 0 {
				*input = (*input)[0 : inputLen-1]
			} else if cursor > 0 {
				min := Max(cursor, 0)
				min = Min(min, inputLen-1)
				first := runes[:min-1]
				last := runes[min:]
				*input = string(first) + string(last)
			}

			return false
		} else if m.Format.CursorY > 0 && m.Format.CursorX == 0 {
			yOffset := Max(m.viewport.YOffset, 0)
			cursor := m.Format.RunningOffsets[m.Format.CursorY+yOffset] + m.Format.CursorX
			runes := []rune(m.selectionText)
			min := Max(cursor, 0)
			min = Min(min, len(m.selectionText)-1)
			first := runes[:min-1]
			last := runes[min:]
			m.selectionText = string(first) + string(last)
			if yOffset+m.viewport.Height == len(m.Format.Text) && yOffset > 0 {
				m.viewport.YOffset--
			} else {
				m.Format.CursorY--
			}
			m.Format.Text = getFormattedTextBuffer(m)
			m.SetViewSlices()
		}

		return true
	}

	return false
}

func handleFormatMode(m *TuiModel, str string) {
	var (
		val         string
		replacement string
	)

	inputReturn := handleFormatInput(m, str)

	if handleFormatMovement(m, str) {
		return
	}

	for _, v := range inputBlacklist {
		if str == v {
			return
		}
	}

	lineNumberOffset := getOffsetForLineNumber(m.Format.CursorY)

	pString := m.Format.Slices[m.Format.CursorY]
	delta := 1
	if str != "backspace" {
		// update UI
		if *pString != "" {
			min := Max(m.Format.CursorX+lineNumberOffset+1, 0)
			min = Min(min, len(*pString))
			first := (*pString)[:min]
			last := (*pString)[min:]
			val = first + str + last
		} else {
			val = *pString + str
		}
	} else {
		delta = -1
		val = *pString
	}

	// if json special rules
	replacement = m.selectionText
	cursor := m.Format.RunningOffsets[m.viewport.YOffset+m.Format.CursorY]

	fIndex := Max(cursor, 0)
	lIndex := m.viewport.YOffset + m.Format.CursorY + 1

	defer func() {
		if recover() != nil {
			println("whoopsy!") // bug happened once, debug...
		}
	}()

	first := replacement[:fIndex]
	middle := val[lineNumberOffset+1:]
	last := replacement[Min(m.Format.RunningOffsets[lIndex], len(replacement)):]

	if (first != "" || last != "") && last != "\n" {
		middle += "\n"
	}

	replacement = first + // replace the entire line the edit appears on
		middle + // insert the edit
		last // top the edit off with the rest of the string

	m.selectionText = replacement
	if len(*pString) == formatModeOffset && str != "backspace" { // insert on empty lines behaves funny
		*pString = *pString + str
	} else {
		*pString = val
	}

	m.Format.CursorX += delta

	if inputReturn {
		return
	}

	for i := m.viewport.YOffset + m.Format.CursorY + 1; i < len(m.Format.RunningOffsets); i++ {
		m.Format.RunningOffsets[i] += delta
	}

}

// handleEditMode implementation is kind of jank, but we can clean it up later
func handleEditMode(m *TuiModel, str string) {
	var (
		input string
		val   string
	)
	selectedInput := &m.textInput.Model
	input = selectedInput.Value()
	if input != "" && selectedInput.Cursor() <= len(input)-1 {
		min := Max(selectedInput.Cursor(), 0)
		min = Min(min, len(input)-1)
		first := input[:min]
		last := input[min:]
		val = first + str + last
	} else {
		val = input + str
	}

	if str == "esc" {
		selectedInput.SetValue("")
		return
	}

	if handleEditMovement(m, str, val) || handleEditInput(m, str, val) {
		return
	}

	for _, v := range inputBlacklist {
		if str == v {
			return
		}
	}

	prePos := selectedInput.Cursor()
	if val != "" {
		selectedInput.SetValue(val)
	} else {
		selectedInput.SetValue(str)
	}

	if prePos != 0 {
		prePos = selectedInput.Cursor()
	}
	selectedInput.setCursor(prePos + 1)
}