# File Translator
Google ai 호출로 지정한 디렉토리에 있는 특정 확장자 파일을 모두 수정해서 다시 저장해 준다.
파일이름을 지정한 경우 해당 파일이 업데이트 된다. 여러 파일도 지정가능.

예제는 mybatis로 되어 있고, prompt.txt를 불러들여서 적용하므로 다른 형태의 파일에도 적용가능.

# 사용법
프로그램은 go run 명령어로 실행하며, `GEMINI_API_KEY`환경변수에서 API키를 가져옵니다.

기본적인 사용 형식은 다음과 같습니다.

```sh

go run main.go [OPTIONS] [PATH_1] [PATH_2] ...
```
   - [OPTIONS]: 실행 옵션을 지정하는 플래그들입니다. (아래 '커맨드 라인 옵션' 참조)
   - [PATH_...]: 처리할 파일 또는 디렉토리의 경로입니다. 공백으로 구분하여 여러 개를 지정할 수 있습니다.

## 실제 예제
```sh
GEMINI_API_KEY=[키] \
    go run main.go \
    -model gemini-2.5-flash-preview-05-20 \
     ../../mybatis/sql \
     ../mybatis/targetfile.xml target2.xml
```

## 사용 가능한 플래그
 | 플래그|	설명 | 기본값 |
 |---------|----------|------------|
 | -model | 사용할 Gemini 모델의 이름을 지정합니다. | gemini-1.5-pro-latest |
 | -prompt | 사용할 프롬프트 파일의 경로를 지정합니다. | prompt.txt |
 | -ext | 처리할 대상 파일의 확장자를 지정합니다. | .xml | 
 | -conflict | Git 충돌 해결 전략을 지정합니다 ('ours' 또는 'theirs'). | theirs |
 
