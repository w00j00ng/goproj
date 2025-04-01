package main

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"
)

func main() {
	// 인자가 없으면 사용법 출력
	if len(os.Args) < 2 {
		fmt.Println("Usage: ./main.out YYYY-MM-DD")
		return
	}

	inputDate := os.Args[1]
	// 날짜 파싱: layout은 "2006-01-02" 형식
	t, err := time.Parse("2006-01-02", inputDate)
	if err != nil {
		fmt.Println("Invalid date format. Use YYYY-MM-DD")
		return
	}

	// 생년월일을 YYMMDD 형식으로 변환
	firstPart := t.Format("060102")

	// 랜덤 시드 초기화
	rand.Seed(time.Now().UnixNano())

	// 주민등록번호 뒷자리 7자리 중 첫번째 숫자는 생년월일에 따른 성별/세기 구분
	var genderDigit int
	if t.Year() < 2000 {
		// 1900년대 출생인 경우: 1(남자) 또는 2(여자)
		genderDigit = rand.Intn(2) + 1 // 1 또는 2
	} else {
		// 2000년대 출생인 경우: 3(남자) 또는 4(여자)
		genderDigit = rand.Intn(2) + 3 // 3 또는 4
	}

	// 첫번째 뒷자리 생성 (성별/세기 구분)
	secondPartDigits := fmt.Sprintf("%d", genderDigit)
	// 그 다음 5자리는 0~9 사이의 임의 숫자 생성
	for i := 0; i < 5; i++ {
		secondPartDigits += strconv.Itoa(rand.Intn(10))
	}

	// 12자리까지 결합 (앞 6자리 + 뒷자리 6자리)
	twelveDigits := firstPart + secondPartDigits

	// 체크섯 계산: 각 자리수에 가중치 곱한 합계 구하기
	// 가중치: [2,3,4,5,6,7,8,9,2,3,4,5]
	weights := []int{2, 3, 4, 5, 6, 7, 8, 9, 2, 3, 4, 5}
	sum := 0
	for i, ch := range twelveDigits {
		digit, _ := strconv.Atoi(string(ch))
		sum += digit * weights[i]
	}
	remainder := sum % 11
	checkDigit := (11 - remainder) % 10

	// 최종 뒷자리 완성 (생성된 6자리 + 체크섯)
	fullSecondPart := secondPartDigits + strconv.Itoa(checkDigit)

	// 주민등록번호: "YYMMDD-XXXXXXX" 형식
	rrn := firstPart + "-" + fullSecondPart
	fmt.Println(rrn)
}
