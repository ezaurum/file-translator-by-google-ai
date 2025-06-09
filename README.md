# File Translator
Google ai 호출로 지정한 디렉토리에 있는 모든 xml 파일을 수정해서 다시 저장해 준다.
파일이름을 지정한 경우 해당 파일이 업데이트 된다. 여러 파일도 지정가능.

예제는 mybatis로 되어 있고, prompt.txt를 불러들여서 적용하므로 다른 형태의 파일에도 적용가능.

```sh
GEMINI_API_KEY=[키] go run main.go -model gemini-2.5-flash-preview-05-20 ../../mybatis/sql ../mybatis/targetfile.xml target2.xml
```

 ----------
 | 플래그|	설명 | 기본값
 | -model | 사용할 Gemini 모델의 이름을 지정합니다. | gemini-1.5-pro-latest
 | -prompt | 사용할 프롬프트 파일의 경로를 지정합니다. | prompt.txt
 | -ext | 처리할 대상 파일의 확장자를 지정합니다. | .xml
 | -conflict | Git 충돌 해결 전략을 지정합니다 ('ours' 또는 'theirs').|theirs
 ---------------------
