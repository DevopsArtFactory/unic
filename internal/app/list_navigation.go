package app

func previousListIndex(index, length int) int {
	if length <= 0 {
		return 0
	}
	if index <= 0 || index >= length {
		return length - 1
	}
	return index - 1
}

func nextListIndex(index, length int) int {
	if length <= 0 {
		return 0
	}
	if index < 0 || index >= length-1 {
		return 0
	}
	return index + 1
}

func clampListIndex(index, length int) int {
	if length <= 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= length {
		return length - 1
	}
	return index
}
