package prompts

import "embed"

//go:embed *.txt
var templateFS embed.FS

type Templates struct {
	Notebook string
	Memo     string
	BlogPost string
	Homepage string
}

func Load() (*Templates, error) {
	notebook, err := templateFS.ReadFile("notebooks.txt")
	if err != nil {
		return nil, err
	}
	memo, err := templateFS.ReadFile("memos.txt")
	if err != nil {
		return nil, err
	}
	blogpost, err := templateFS.ReadFile("blogposts.txt")
	if err != nil {
		return nil, err
	}
	homepage, err := templateFS.ReadFile("homepage.txt")
	if err != nil {
		return nil, err
	}

	return &Templates{
		Notebook: string(notebook),
		Memo:     string(memo),
		BlogPost: string(blogpost),
		Homepage: string(homepage),
	}, nil
}
