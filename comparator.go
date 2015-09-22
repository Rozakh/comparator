package comparator

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"
)

//Diff type constants.
const (
	Delete DiffType = -1
	Insert DiffType = 1
)

var (
	jsonDiffer *gojsondiff.Differ
	textDiffer *diffmatchpatch.DiffMatchPatch
)

//Diff includes text difference and diff type.
type Diff struct {
	Text string
	Type DiffType
}

//DiffType is a type of the difference(insert or delete).
type DiffType int8

func init() {
	jsonDiffer = gojsondiff.New()
	textDiffer = diffmatchpatch.New()
}

//Compare responses for the provided urls. Compare only specified html elements or compare responses as json if
//elements are not provided.
func Compare(aURL, bURL string, compareElements []string) ([]Diff, error) {
	aResp, aErr := http.Get(aURL)
	bResp, bErr := http.Get(bURL)
	if aErr != nil && bErr == nil {
		err := trimErrorHost(aErr)
		return []Diff{Diff{err.Error(), Delete}, Diff{bResp.Status, Insert}}, nil
	}
	if aErr == nil && bErr != nil {
		err := trimErrorHost(bErr)
		return []Diff{Diff{aResp.Status, Delete}, Diff{err.Error(), Insert}}, nil
	}
	if aErr != nil && bErr != nil {
		aError := trimErrorHost(aErr)
		bError := trimErrorHost(bErr)
		return compareStrings(aError.Error(), bError.Error()), nil
	}
	if compareElements == nil {
		return compareJSONs(aResp, bResp)
	}
	return compareHTMLs(aResp, bResp, compareElements)
}

func compareJSONs(aResp, bResp *http.Response) ([]Diff, error) {
	var aJSON map[string]interface{}
	defer aResp.Body.Close()
	defer bResp.Body.Close()
	aBody, err := ioutil.ReadAll(aResp.Body)
	if err != nil {
		return nil, err
	}
	bBody, err := ioutil.ReadAll(bResp.Body)
	if err != nil {
		return nil, err
	}
	diff, err := jsonDiffer.Compare(aBody, bBody)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(aBody, &aJSON)
	formatter := formatter.NewAsciiFormatter(aJSON)
	diffString, err := formatter.Format(diff)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(diffString, "\n")
	return getDiffsFromStrings(lines), nil
}

func compareHTMLs(aResp, bResp *http.Response, compareElements []string) ([]Diff, error) {
	var result []Diff
	aDoc, err := goquery.NewDocumentFromResponse(aResp)
	if err != nil {
		return nil, err
	}
	bDoc, err := goquery.NewDocumentFromResponse(bResp)
	if err != nil {
		return nil, err
	}
	for _, element := range compareElements {
		aElement := aDoc.Find(element)
		bElement := bDoc.Find(element)
		result = append(result, compareStrings(aElement.Text(), bElement.Text())...)
	}
	return result, nil
}

func compareStrings(aString, bString string) []Diff {
	var result []Diff
	diffs := textDiffer.DiffMain(aString, bString, true)
	diffs = textDiffer.DiffCleanupSemantic(diffs)
	for _, element := range diffs {
		if element.Type == diffmatchpatch.DiffInsert {
			result = append(result, Diff{element.Text, Insert})
		} else if element.Type == diffmatchpatch.DiffDelete {
			result = append(result, Diff{element.Text, Delete})
		}
	}
	return result
}

func getDiffsFromStrings(lines []string) []Diff {
	var diffs []Diff
	for _, line := range lines {
		if strings.HasPrefix(line, "+") {
			line = strings.Replace(line, "+", "", 1)
			diffs = append(diffs, Diff{line, Insert})
		} else if strings.HasPrefix(line, "-") {
			line = strings.Replace(line, "-", "", 1)
			diffs = append(diffs, Diff{line, Delete})
		} else {
			continue
		}
	}
	return diffs
}

func trimErrorHost(err error) error {
	errText := err.Error()
	errWithoutHost := errText[strings.LastIndex(errText, ":"):len(errText)]
	return errors.New(errWithoutHost)
}
