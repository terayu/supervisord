package ini

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

// remove inline comments
//
// inline comments must start with ';' or '#'
// and the char before the ';' or '#' must be a space
//
func removeComments(value string) string {
	n := len(value)
	i := 0
	for ; i < n; i++ {
		if value[i] == '\\' {
			i++
		} else if value[i] == ';' || value[i] == '#' {
			if i > 0 && unicode.IsSpace(rune(value[i-1])) {
				return strings.TrimSpace(value[0:i])
			}
		}
	}
	return strings.TrimSpace(value)
}

func removeContinuationSuffix(value string) (string, bool) {
	pos := strings.LastIndex(value, "\\")
	n := len(value)
	if pos == -1 || pos != n-1 {
		return "", false
	}
	for pos >= 0 {
		if value[pos] != '\\' {
			return "", false
		}
		pos--
		if pos < 0 || value[pos] != '\\' {
			return value[0 : n-1], true
		}
		pos--
	}
	return "", false
}

type lineReader struct {
	reader *bufio.Scanner
}

func newLineReader(reader io.Reader) *lineReader {
	return &lineReader{reader: bufio.NewScanner(reader)}
}

func (lr *lineReader) readLine() (string, error) {
	if lr.reader.Scan() {
		return lr.reader.Text(), nil
	}
	return "", errors.New("No data")

}

func readLinesUntilSuffix(lineReader *lineReader, suffix string) string {
	r := ""
	for {
		line, err := lineReader.readLine()
		if err != nil {
			break
		}
		t := strings.TrimRightFunc(line, unicode.IsSpace)
		if strings.HasSuffix(t, suffix) {
			r = r + t[0:len(t)-len(suffix)]
			break
		} else {
			r = r + line + "\n"
		}
	}
	return r
}

// if a line enss with char '\', we can read the next line
//
func readContinuationLines(lineReader *lineReader) string {
	r := ""
	for {
		line, err := lineReader.readLine()
		if err != nil {
			break
		}
		line = strings.TrimRightFunc(line, unicode.IsSpace)
		if t, continuation := removeContinuationSuffix(line); continuation {
			r = r + t
		} else {
			r = r + line
			break
		}
	}
	return r
}

/*
Load from the sources, the source can be one of:
    - fileName
    - a string includes .ini
    - io.Reader the reader to load the .ini contents
    - byte array incldues .ini content
*/
func (ini *Ini) Load(sources ...interface{}) {
	for _, source := range sources {
		switch source.(type) {
		case string:
			s, _ := source.(string)
			if _, err := os.Stat(s); err == nil {
				ini.LoadFile(s)
			} else {
				ini.LoadString(s)
			}
		case io.Reader:
			reader, _ := source.(io.Reader)
			ini.LoadReader(reader)
		case []byte:
			b, _ := source.([]byte)
			ini.LoadBytes(b)
		}
	}

}

// return the number of spaces before non-space chars
func getIndent(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if unicode.IsSpace(rune(s[i])) {
			n++
		} else {
			break
		}
	}
	return n
}

func isCommentLine(line string) bool {
	line = strings.TrimSpace(line)
	return len(line) <= 0 || line[0] == ';' || line[0] == '#'
}

// parse the section if it is a section line
// Return section name if it is a section else return nil
func parseSectionName(line string) *string {
	line = strings.TrimSpace(line)

	if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
		sectionName := strings.TrimSpace(line[1 : len(line)-1])
		return &sectionName
	} else {
		return nil
	}

}

// Explicitly loads .ini from a reader
//
func (ini *Ini) LoadReader(reader io.Reader) {
	lineReader := newLineReader(reader)
	var curSection *Section = nil
	keyIndent := -1
	prevKey := ""
	for {
		line, err := lineReader.readLine()
		if err != nil {
			break
		}

		//if this line is value of the key
		if keyIndent >= 0 && getIndent(line) > keyIndent && curSection != nil && prevKey != "" {
			v := curSection.GetValueWithDefault(prevKey, "")
			v = fmt.Sprintf("%s\n%s", v, removeComments(line))
			curSection.Add(prevKey, v )

			continue
		}

		//empty line or comments line
		if isCommentLine(line) {
			continue
		}
		//if it is a section
		sectionName := parseSectionName( line )
		if sectionName != nil {
			curSection = ini.NewSection(*sectionName)
			// reset the previous key and the key indent
			prevKey = ""
			keyIndent = -1
			continue
		}
		//key&value is separated with = or :
		pos := strings.IndexAny(line, "=:")
		if pos != -1 {
			keyIndent = getIndent(line)
			key := strings.TrimSpace(line[0:pos])
			prevKey = key
			value := strings.TrimLeftFunc(line[pos+1:], unicode.IsSpace)
			//if it is a multiline indicator """
			if strings.HasPrefix(value, "\"\"\"") {
				t := strings.TrimRightFunc(value, unicode.IsSpace)
				//if the end multiline indicator is found
				if strings.HasSuffix(t, "\"\"\"") {
					value = t[3 : len(t)-3]
				} else { //read lines until end multiline indicator is found
					value = value[3:] + "\n" + readLinesUntilSuffix(lineReader, "\"\"\"")
				}
			} else {
				value = strings.TrimRightFunc(value, unicode.IsSpace)
				//if is it a continuation line
				if t, continuation := removeContinuationSuffix(value); continuation {
					value = t + readContinuationLines(lineReader)
				}
			}

			if len(key) > 0 {
				if curSection == nil && len(ini.defaultSectionName) > 0 {
					curSection = ini.NewSection(ini.defaultSectionName)
				}
				if curSection != nil {
					//remove the comments
					curSection.Add(key, strings.TrimSpace(removeComments(value)))
				}
			}
		}
	}
}

// Load ini file from file named fileName
//
func (ini *Ini) LoadFile(fileName string) {
	f, err := os.Open(fileName)
	if err == nil {
		defer f.Close()
		ini.Load(f)
	}
}

var defaultSectionName string = "default"

func SetDefaultSectionName(defSectionName string) {
	defaultSectionName = defSectionName
}

// load ini from the content which contains the .ini formated string
//
func (ini *Ini) LoadString(content string) {
	ini.Load(bytes.NewBufferString(content))
}

// load .ini from a byte array which contains the .ini formated content
func (ini *Ini) LoadBytes(content []byte) {
	ini.Load(bytes.NewBuffer(content))
}

/*
Load the .ini from one of following resource:
    - file
    - string in .ini format
    - byte array in .ini format
    - io.Reader a reader to load .ini content

One or more source can be provided in this Load method, such as:
    var reader1 io.Reader = ...
    var reader2 io.Reader = ...
    ini.Load( "./my.ini", "[section]\nkey=1", "./my2.ini", reader1, reader2 )
*/
func Load(sources ...interface{}) *Ini {
	ini := NewIni()
	ini.SetDefaultSectionName(defaultSectionName)
	for _, source := range sources {
		ini.Load(source)
	}
	return ini
}
