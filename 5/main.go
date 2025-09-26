package main

type customError struct {
	msg string
}

func (e *customError) Error() string {
	return e.msg
}

func test() *customError {
	return nil
}

func main() {
	var err error
	err = test() // интерфейс считается nil только если и тип, и значение внутри него равны nil, а у нас
	// переменная err принимает значение nil, тип интерфейса *customError

	if err != nil {
		println("error")
		return
	}
	println("ok")
}
