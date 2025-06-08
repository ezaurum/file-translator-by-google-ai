package main

import (
	"context"
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

// --- 프롬프트 상수는 이제 사용하지 않음 ---
// const promptFormat = `...`

// --- 코드 본문 ---

func main() {
	// --- 1. 기본 설정 및 초기화 ---
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("환경변수 'GEMINI_API_KEY'가 설정되지 않았습니다.")
	}
	if len(os.Args) < 2 {
		log.Fatal("쿼리를 수정할 MyBatis 파일이 있는 디렉토리 경로를 입력해주세요.\n사용법: go run main.go /path/to/your/repo")
	}
	rootDir := os.Args[1]

	// --- NEW: 외부 프롬프트 파일 읽기 ---
	promptBytes, err := os.ReadFile("prompt.txt")
	if err != nil {
		log.Fatalf("프롬프트 파일(prompt.txt)을 읽는 중 오류 발생: %v", err)
	}
	promptTemplate := string(promptBytes) // 읽어온 내용을 템플릿으로 사용

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("Gemini 클라이언트 생성 실패: %v", err)
	}
	defer client.Close()

	modelName := "gemini-2.5-pro-preview-0605"
	model := client.GenerativeModel(modelName)
	model.SetTemperature(0.1)

	// --- 2. 처리할 파일 목록 수집 ---
	log.Println("처리할 .xml 파일을 수집 중입니다...")
	var filesToProcess []string
	err = filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".xml") {
			filesToProcess = append(filesToProcess, path)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("파일 목록을 가져오는 중 오류 발생: %v", err)
	}

	totalFiles := len(filesToProcess)
	if totalFiles == 0 {
		log.Println("처리할 .xml 파일을 찾지 못했습니다.")
		return
	}

	// --- 3. 예상 시간 계산 및 시작 메시지 ---
	estimatedTotalSeconds := totalFiles * (estimatedApiTimeSeconds + sleepDelaySeconds)
	estimatedDuration := time.Duration(estimatedTotalSeconds) * time.Second
	log.Printf("총 %d개의 .xml 파일을 찾았습니다.", totalFiles)
	log.Printf("예상 소요 시간: 약 %s", formatDuration(estimatedDuration))
	log.Println("작업을 시작합니다...")

	// --- 4. 파일 처리 루프 및 진행률 표시 ---
	processedCount := 0
	startTime := time.Now()

	for _, path := range filesToProcess {
		// --- CHANGED: processFile 호출 시 promptTemplate 전달 ---
		if err := processFile(ctx, model, path, promptTemplate); err != nil {
			// 오류 로그는 processFile 내부에서 처리
		}

		processedCount++

		elapsed := time.Since(startTime)
		progress := float64(processedCount) / float64(totalFiles)

		avgTimePerFile := elapsed / time.Duration(processedCount)
		remainingFiles := totalFiles - processedCount
		remainingTime := avgTimePerFile * time.Duration(remainingFiles)

		fmt.Printf("\r[%d/%d] %.1f%% 완료 | 경과 시간: %s | 남은 예상 시간: %s",
			processedCount,
			totalFiles,
			progress*100,
			formatDuration(elapsed),
			formatDuration(remainingTime),
		)

		if processedCount < totalFiles {
			time.Sleep(sleepDelaySeconds * time.Second)
		}
	}

	// --- 5. 최종 결과 출력 ---
	fmt.Println()
	log.Println("모든 작업이 완료되었습니다.")
	log.Printf("총 경과 시간: %s", formatDuration(time.Since(startTime)))
}

// --- CHANGED: processFile 함수가 promptTemplate을 인자로 받도록 수정 ---
func processFile(ctx context.Context, model *genai.GenerativeModel, filePath string, promptTemplate string) error {
	originalContent, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("\n오류: %s 파일 읽기 실패: %v", filePath, err)
		return err
	}
	if len(originalContent) == 0 {
		return nil
	}

	// --- CHANGED: 인자로 받은 템플릿을 사용 ---
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
	modifiedContent, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		log.Printf("\n오류: %s 파일의 Gemini 응답이 텍스트 형식이 아닙니다.", filePath)
		return fmt.Errorf("response is not text")
	}
	err = os.WriteFile(filePath, []byte(modifiedContent), 0644)
	if err != nil {
		log.Printf("\n오류: %s 파일 쓰기 실패: %v", filePath, err)
		return err
	}
	return nil
}

// formatDuration 함수는 이전과 동일
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	return fmt.Sprintf("%d분 %d초", m, s)
}
