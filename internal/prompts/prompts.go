package prompts

import "embed"

//go:embed *.txt
var templateFS embed.FS

type Templates struct {
	BlogPost  string
	Humanizer string
}

func Load() (*Templates, error) {
	blogpost, err := templateFS.ReadFile("blogposts.txt")
	if err != nil {
		return nil, err
	}
	humanizer, err := templateFS.ReadFile("humanizer.txt")
	if err != nil {
		return nil, err
	}
	return &Templates{BlogPost: string(blogpost), Humanizer: string(humanizer)}, nil
}
