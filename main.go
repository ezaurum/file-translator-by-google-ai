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

func processFile(ctx context.Context, model *genai.GenerativeModel, filePath string, promptTemplate string) error {
	originalContent, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("\n오류: %s 파일 읽기 실패: %v", filePath, err)
		return err
	}
	if len(originalContent) == 0 {
		return nil
	}
	prompt := fmt.Sprintf(promptTemplate, string(originalContent))
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
	var modelName string
	flag.StringVar(&modelName, "model", "gemini-1.5-pro-latest", "사용할 Gemini 모델의 이름을 지정합니다 (e.g., gemini-1.5-flash-latest)")
	flag.Parse()

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("환경변수 'GEMINI_API_KEY'가 설정되지 않았습니다.")
	}
	if flag.NArg() < 1 {
		log.Fatal("하나 이상의 대상 파일 또는 디렉토리 경로를 입력해주세요.\n사용법: go run main.go [-model 모델이름] [파일/디렉토리 경로] ...")
	}

	promptBytes, err := os.ReadFile("prompt.txt")
	if err != nil {
		log.Fatalf("프롬프트 파일(prompt.txt)을 읽는 중 오류 발생: %v", err)
	}
	promptTemplate := string(promptBytes)

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
				if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".xml") {
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
		log.Println("처리할 .xml 파일을 찾지 못했습니다.")
		return
	}
	
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
	model := client.GenerativeModel(modelName)
	model.SetTemperature(0.1)

	estimatedTotalSeconds := totalFiles * (estimatedApiTimeSeconds + sleepDelaySeconds)
	estimatedDuration := time.Duration(estimatedTotalSeconds) * time.Second
	log.Printf("총 %d개의 고유한 파일을 처리합니다.", totalFiles)
	log.Printf("예상 소요 시간: 약 %s", formatDuration(estimatedDuration))
	log.Println("작업을 시작합니다...")

	processedCount := 0
	startTime := time.Now()

	for _, path := range filesToProcess {
		// --- CHANGED: 파일 처리 전에 진행률을 먼저 표시하도록 순서 변경 ---
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

		// 이제 화면에 "처리중"이 표시된 상태에서 아래 함수가 실행됩니다.
		if err := processFile(ctx, model, path, promptTemplate); err != nil {
			// 오류 로그는 processFile 내부에서 처리되므로 여기서는 별도 처리가 필요 없습니다.
		}

		// 마지막 파일 처리가 끝난 후에는 대기하지 않습니다.
		if processedCount < totalFiles {
			time.Sleep(sleepDelaySeconds * time.Second)
		}
	}
	
	fmt.Println() 
	log.Println("모든 작업이 완료되었습니다.")
	log.Printf("총 경과 시간: %s", formatDuration(time.Since(startTime)))
}