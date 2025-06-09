package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// --- 설정값 ---
const sleepDelaySeconds = 5
const estimatedApiTimeSeconds = 3
const maxPathDisplayLength = 60

// --- Helper Functions (변경 없음) ---

func resolveGitConflict(content string, strategy string) (resolvedContent string, wasConflicted bool) {
	if !strings.Contains(content, "<<<<<<< HEAD") || !strings.Contains(content, "=======") {
		return content, false
	}
	lines := strings.Split(content, "\n")
	var resultLines []string
	var inConflictBlock, shouldKeep bool
	for _, line := range lines {
		if strings.HasPrefix(line, "<<<<<<< HEAD") {
			inConflictBlock = true
			shouldKeep = (strategy == "ours")
			continue
		}
		if strings.HasPrefix(line, "=======") && inConflictBlock {
			shouldKeep = (strategy == "theirs")
			continue
		}
		if strings.HasPrefix(line, ">>>>>>>") && inConflictBlock {
			inConflictBlock = false
			shouldKeep = false
			continue
		}
		if !inConflictBlock || shouldKeep {
			resultLines = append(resultLines, line)
		}
	}
	return strings.Join(resultLines, "\n"), true
}

func truncatePath(path string, maxLength int) string {
	if len(path) <= maxLength {
		return path
	}
	return "..." + path[len(path)-(maxLength-3):]
}

func cleanApiResponse(response string) string {
	cleaned := strings.TrimSpace(response)
	if strings.HasPrefix(cleaned, "```xml") && strings.HasSuffix(cleaned, "```") {
		cleaned = strings.TrimPrefix(cleaned, "```xml")
		cleaned = strings.TrimSuffix(cleaned, "```")
		cleaned = strings.TrimSpace(cleaned)
		return cleaned
	}
	if strings.HasPrefix(cleaned, "```") && strings.HasSuffix(cleaned, "```") {
		cleaned = strings.TrimPrefix(cleaned, "```")
		cleaned = strings.TrimSuffix(cleaned, "```")
		cleaned = strings.TrimSpace(cleaned)
		return cleaned
	}
	return cleaned
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	return fmt.Sprintf("%d분 %d초", m, s)
}

func processFile(ctx context.Context, model *genai.GenerativeModel, filePath string, promptTemplate string, conflictStrategy string) error {
	originalContent, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("\n오류: %s 파일 읽기 실패: %v", filePath, err)
		return err
	}
	if len(originalContent) == 0 {
		return nil
	}
	stringContent := string(originalContent)
	resolvedContent, wasConflicted := resolveGitConflict(stringContent, conflictStrategy)
	if wasConflicted {
		log.Printf("\n알림: %s 파일에서 Git 충돌을 발견하여 '%s' 버전으로 자동 해결했습니다.", filePath, conflictStrategy)
	}
	prompt := fmt.Sprintf(promptTemplate, resolvedContent)
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		log.Printf("\n오류: %s 파일의 Gemini API 호출 실패: %v", filePath, err)
		return err
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		log.Printf("\n오류: %s 파일에 대해 Gemini로부터 유효한 응답을 받지 못했습니다.", filePath)
		return fmt.Errorf("empty response from API")
	}
	rawResponse, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		log.Printf("\n오류: %s 파일의 Gemini 응답이 텍스트 형식이 아닙니다.", filePath)
		return fmt.Errorf("response is not text")
	}
	finalContent := cleanApiResponse(string(rawResponse))
	err = os.WriteFile(filePath, []byte(finalContent), 0644)
	if err != nil {
		log.Printf("\n오류: %s 파일 쓰기 실패: %v", filePath, err)
		return err
	}
	return nil
}

// --- main 함수 ---
func main() {
	// --- 1. 플래그 정의 ---
	var modelName, conflictStrategy, promptFile, fileExt string
	
	flag.StringVar(&modelName, "model", "gemini-1.5-pro-latest", "사용할 Gemini 모델의 이름을 지정합니다.")
	// CHANGED: 기본값을 'theirs'로 변경
	flag.StringVar(&conflictStrategy, "conflict", "theirs", "Git 충돌 해결 전략을 지정합니다 ('ours' 또는 'theirs').")
	// NEW: 프롬프트 파일을 지정하는 플래그
	flag.StringVar(&promptFile, "prompt", "prompt.txt", "사용할 프롬프트 파일의 경로를 지정합니다.")
	// NEW: 파일 확장자를 지정하는 플래그
	flag.StringVar(&fileExt, "ext", ".xml", "처리할 대상 파일의 확장자를 지정합니다 (e.g., .sql).")
	
	flag.Parse()

	// --- 2. 설정값 검증 및 처리 ---
	if conflictStrategy != "ours" && conflictStrategy != "theirs" {
		log.Fatal("-conflict 플래그는 'ours' 또는 'theirs'만 허용됩니다.")
	}
	// NEW: 사용자가 점(.)을 빼먹어도 자동으로 추가
	if !strings.HasPrefix(fileExt, ".") {
		fileExt = "." + fileExt
	}
	
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("환경변수 'GEMINI_API_KEY'가 설정되지 않았습니다.")
	}
	if flag.NArg() < 1 {
		log.Fatal("하나 이상의 대상 파일 또는 디렉토리 경로를 입력해주세요.\n사용법: go run main.go [옵션...] [파일/디렉토리 경로] ...")
	}
	
	// CHANGED: 플래그로 받은 프롬프트 파일 경로 사용
	promptBytes, err := os.ReadFile(promptFile)
	if err != nil {
		log.Fatalf("프롬프트 파일(%s)을 읽는 중 오류 발생: %v", promptFile, err)
	}
	promptTemplate := string(promptBytes)

	// --- 3. 처리할 파일 목록 수집 ---
	log.Println("처리할 파일 목록을 수집 중입니다...")
	var filesToProcess []string
	for _, pathArg := range flag.Args() {
		info, err := os.Stat(pathArg)
		if err != nil {
			log.Printf("경고: 경로를 확인할 수 없습니다. 건너뜁니다: %s (%v)", pathArg, err)
			continue
		}
		if info.IsDir() {
			err = filepath.Walk(pathArg, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				// CHANGED: 플래그로 받은 확장자 사용
				if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), fileExt) {
					filesToProcess = append(filesToProcess, path)
				}
				return nil
			})
			if err != nil {
				log.Printf("경고: 디렉토리 스캔 중 오류 발생: %s (%v)", pathArg, err)
			}
		} else {
			filesToProcess = append(filesToProcess, pathArg)
		}
	}

	seen := make(map[string]bool)
	uniqueFiles := []string{}
	for _, file := range filesToProcess {
		if _, ok := seen[file]; !ok {
			seen[file] = true
			uniqueFiles = append(uniqueFiles, file)
		}
	}
	filesToProcess = uniqueFiles

	totalFiles := len(filesToProcess)
	if totalFiles == 0 {
		log.Printf("처리할 %s 파일을 찾지 못했습니다.", fileExt)
		return
	}
	
	// --- 4. Gemini 클라이언트 초기화 및 작업 시작 ---
	ctx := context.Background()
	client, err := genai.NewClient(ctx,
		option.WithAPIKey(apiKey),
		option.WithEndpoint("generativelanguage.googleapis.com"),
	)
	if err != nil {
		log.Fatalf("Gemini 클라이언트 생성 실패: %v", err)
	}
	defer client.Close()
	
	log.Printf("사용할 모델: %s", modelName)
	log.Printf("Git 충돌 해결 전략: %s", conflictStrategy)
	log.Printf("사용할 프롬프트: %s", promptFile)
	log.Printf("대상 확장자: %s", fileExt)
	
	model := client.GenerativeModel(modelName)
	model.SetTemperature(0.1)

	estimatedTotalSeconds := totalFiles * (estimatedApiTimeSeconds + sleepDelaySeconds)
	estimatedDuration := time.Duration(estimatedTotalSeconds) * time.Second
	log.Printf("총 %d개의 고유한 파일을 처리합니다.", totalFiles)
	log.Printf("예상 소요 시간: 약 %s", formatDuration(estimatedDuration))
	log.Println("작업을 시작합니다...")

	// --- 5. 파일 처리 루프 ---
	processedCount := 0
	startTime := time.Now()

	for _, path := range filesToProcess {
		processedCount++
		elapsed := time.Since(startTime)
		progress := float64(processedCount) / float64(totalFiles)
		
		displayPath := truncatePath(path, maxPathDisplayLength)
		fmt.Printf("\r[%d/%d] %.1f%% | 처리중: %-*s | 경과: %s",
			processedCount,
			totalFiles,
			progress*100,
			maxPathDisplayLength, displayPath,
			formatDuration(elapsed),
		)

		if err := processFile(ctx, model, path, promptTemplate, conflictStrategy); err != nil {
			//
		}

		if processedCount < totalFiles {
			time.Sleep(sleepDelaySeconds * time.Second)
		}
	}
	
	// --- 6. 최종 결과 출력 ---
	fmt.Println() 
	log.Println("모든 작업이 완료되었습니다.")
	log.Printf("총 경과 시간: %s", formatDuration(time.Since(startTime)))
}