package main

import (
	"flag"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

func main() {
	// gender 플래그 정의 (기본값 "male")
	gender := flag.String("gender", "male", "성별 지정: male 혹은 female")
	flag.Parse()

	// 남은 인자에서 생년월일을 가져옴
	if flag.NArg() < 1 {
		fmt.Println("Usage: ./main.out [--gender=male|female] YYYY-MM-DD")
		return
	}

	inputDate := flag.Arg(0)
	// 날짜 파싱: layout은 "2006-01-02" 형식
	t, err := time.Parse("2006-01-02", inputDate)
	if err != nil {
		fmt.Println("Invalid date format. Use YYYY-MM-DD")
		return
	}

	// 생년월일을 YYMMDD 형식으로 변환
	firstPart := t.Format("060102")

	// 랜덤 시드 초기화
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 성별에 따른 주민등록번호 뒷자리 첫번째 숫자 결정
	genderDigit := 1
	if gender != nil {
		switch strings.ToLower(*gender) {
		case "male", "m", "man":
			if t.Year() < 2000 {
				genderDigit = 1 // 1900년대 남성
			} else {
				genderDigit = 3 // 2000년대 남성
			}
		case "female", "f", "woman":
			if t.Year() < 2000 {
				genderDigit = 2 // 1900년대 여성
			} else {
				genderDigit = 4 // 2000년대 여성
			}
		default:
			genderDigit = 1 // 1900년대 남성
		}
	}

	// 뒷자리 첫번째 숫자 설정 후, 나머지 5자리 임의 숫자 생성
	secondPartDigits := fmt.Sprintf("%d", genderDigit)
	for i := 0; i < 5; i++ {
		secondPartDigits += strconv.Itoa(r.Intn(10))
	}

	// 앞 12자리 (6자리 생년월일 + 6자리 뒷자리) 생성
	twelveDigits := firstPart + secondPartDigits

	// 체크섯 계산 (가중치: [2,3,4,5,6,7,8,9,2,3,4,5])
	weights := []int{2, 3, 4, 5, 6, 7, 8, 9, 2, 3, 4, 5}
	sum := 0
	for i, ch := range twelveDigits {
		digit, _ := strconv.Atoi(string(ch))
		sum += digit * weights[i]
	}
	remainder := sum % 11
	checkDigit := (11 - remainder) % 10

	// 최종 주민등록번호 생성 ("YYMMDD-XXXXXXX")
	fullSecondPart := secondPartDigits + strconv.Itoa(checkDigit)
	rrn := firstPart + "-" + fullSecondPart
	fmt.Println(rrn)
}