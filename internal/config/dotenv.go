package config

import (
	"bufio"
	"os"
	"strings"
)

// bom 은 윈도우 메모장 등이 파일 맨 앞에 붙이는 보이지 않는 표식이다.
// 떼지 않으면 첫 키 이름이 미묘하게 달라져 "키가 비어 있습니다" 가 뜨는데,
// 편집기에서는 멀쩡해 보이므로 원인을 찾기 어렵다.
const bom = "\xef\xbb\xbf"

// DotEnvPath는 기본으로 읽는 설정 파일이다.
const DotEnvPath = ".env"

// loadDotEnv는 .env 파일을 키-값 맵으로 읽는다.
//
// 라이브러리를 쓰지 않는 이유: 우리가 필요한 문법은 KEY=VALUE 한 줄뿐이고,
// 의존성 하나를 늘리는 것보다 30줄이 낫다. 셸로 source 하는 방법은
// PAYOUT_DEEPLINK_TEMPLATES 의 JSON 값에 & 와 {} 가 들어 있어서 깨진다.
//
// 파일이 없으면 빈 맵과 false 를 반환한다. 에러가 아니다 — 도커나 CI 에서는
// 진짜 환경변수로 넘어온다.
func loadDotEnv(path string) (map[string]string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return map[string]string{}, false
	}
	defer f.Close() //nolint:errcheck

	out := map[string]string{}
	sc := bufio.NewScanner(f)
	first := true

	for sc.Scan() {
		line := sc.Text()

		// 윈도우에서 만든 파일은 줄 끝에 \r 이 붙는다. 그대로 두면
		// 키 값 끝에 보이지 않는 문자가 남아 base64 디코딩이 실패한다.
		line = strings.TrimSuffix(line, "\r")

		if first {
			line = strings.TrimPrefix(line, bom)
			first = false
		}

		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		out[key] = unquote(strings.TrimSpace(val))
	}
	return out, true
}

// unquote는 따옴표를 벗기고, 따옴표가 없으면 뒤따르는 주석을 떼어낸다.
func unquote(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	// 따옴표가 없을 때만 " #" 뒤를 주석으로 본다.
	// 공백 없는 # 는 URL 조각일 수 있으므로 건드리지 않는다.
	if i := strings.Index(v, " #"); i >= 0 {
		v = v[:i]
	}
	return strings.TrimRight(v, " \t")
}

// envWithDotEnv는 진짜 환경변수를 우선하고, 없으면 .env 값을 쓴다.
//
// 이 순서여야 도커 compose 의 environment 와 CI 의 비밀값이 파일을 덮는다.
// 반대로 하면 서버에 남아 있던 개발용 .env 가 운영 설정을 조용히 이긴다.
func envWithDotEnv(file map[string]string) Getenv {
	return func(k string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return file[k]
	}
}
