package prompts

import "embed"

//go:embed *.txt
var templateFS embed.FS

type Templates struct {
	BlogPost     string
	TopicPlanner string
}

func Load() (*Templates, error) {
	blogpost, err := templateFS.ReadFile("blogposts.txt")
	if err != nil {
		return nil, err
	}
	topicPlanner, err := templateFS.ReadFile("topic_planner.txt")
	if err != nil {
		return nil, err
	}
	return &Templates{
		BlogPost:     string(blogpost),
		TopicPlanner: string(topicPlanner),
	}, nil
}
