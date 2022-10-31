package surveyext

import "github.com/AlecAivazis/survey/v2/core"

func paginate(pageSize int, choices []core.OptionAnswer, sel int) (choiceList []core.OptionAnswer, cursor int) {
	var start, end int

	if len(choices) < pageSize {
		// if we dont have enough options to fill a page
		start = 0
		end = len(choices)
		cursor = sel
		choiceList = choices[start:end]

	} else if sel < pageSize/2 {
		// if we are in the first half page
		start = 0
		end = pageSize - 1
		cursor = sel
		choiceList = append(choices[start:end], choices[len(choices)-1])

	} else if len(choices)-sel-1 < pageSize/2 {
		// if we are in the last half page
		start = len(choices) - pageSize
		end = len(choices)
		cursor = sel - start
		choiceList = choices[start:end]
	} else {
		// somewhere in the middle
		above := pageSize / 2
		below := pageSize - above

		cursor = pageSize / 2
		start = sel - above
		end = (sel + below) - 1
		choiceList = append(choices[start:end], choices[len(choices)-1])
	}

	// return the subset we care about and the index
	return choiceList, cursor
}
