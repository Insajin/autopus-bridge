package apiclient

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// PrintTable은 tabwriter를 이용해 정렬된 테이블을 출력합니다.
// headers가 있으면 첫 행으로 출력합니다.
func PrintTable(w io.Writer, headers []string, rows [][]string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer func() { _ = tw.Flush() }()

	if len(headers) > 0 {
		_, _ = fmt.Fprintln(tw, strings.Join(headers, "\t"))
	}
	for _, row := range rows {
		_, _ = fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
}

// PrintJSON은 데이터를 들여쓰기가 적용된 JSON 형식으로 출력합니다.
func PrintJSON(w io.Writer, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON 직렬화 실패: %w", err)
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

// PrintDetail은 키-값 쌍을 정렬된 형식으로 출력합니다.
// 각 필드는 "Key:  Value" 형식으로 출력됩니다.
func PrintDetail(w io.Writer, fields []KeyValue) {
	if len(fields) == 0 {
		return
	}

	// 키 최대 길이 계산
	maxKeyLen := 0
	for _, f := range fields {
		if len(f.Key) > maxKeyLen {
			maxKeyLen = len(f.Key)
		}
	}

	for _, f := range fields {
		// 키를 maxKeyLen으로 패딩하여 값이 정렬되도록 출력
		_, _ = fmt.Fprintf(w, "%-*s  %s\n", maxKeyLen, f.Key+":", f.Value)
	}
}
