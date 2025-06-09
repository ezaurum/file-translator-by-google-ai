# File Translator
Google ai 호출로 해당 디렉토리에 있는 모든 파일을 수정해서 다시 저장해 준다.
파일이름을 지정한 경우 해당 파일이 업데이트 된다. 여러 파일도 지정가


예제는 mybatis로 되어 있고, 프롬프트만 변경하면 모든 경우에 적용가능.

```sh
GEMINI_API_KEY=[키] go run main.go -model gemini-2.5-flash-preview-05-20 ../../mybatis/sql ../mybatis/targetfile.xml target2.xml
```
