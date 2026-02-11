# Blog Content Generation from R&D

We're a startup, named frankenstein ai lab, a startup focused on AI R&D.

We're working on lot's of researches and also do not have time to write properly about these.

We wanna a way to aotomatic do it based on our projects commits

## Automatic Notebooks and Insight Memos from researches work

Our work uses git and we can describe as best the commits message and descriptions, so it helps us to create notebooks and Insight Memos of the work. The goal here is to have a feature that automatic reads the commits starting at some point (we should track the last commit hash we used to generate notebooks and insight memos), and generate new notebooks and insight memos.

### About notebooks and Insight Memos

* Notebooks are about the work did on a time space (day, week, etc)
* Insight Memos are knowledge learned about a topic

## Automatic generate markdown blog post

Our work use markdown to write blog posts. It's a neutral format and allow using any modern blog engine. Today, we read the notebooks and insight memos to write blog posts. The goal here  is to have a feature that automatic reads the commits stating at some point (we should track the last commit we used) and generate blog posts.

## The homepage

The homepage has a up to date info about the job with last researches/work, and WIP (work in progress)

## UX

I wanna have a way to configure the project dir to extract the commits, the project dir of insight memos and notebooks and the blog dir.

## Teck stack

I think it can be a cli written in golang, using some good text generation LLM and allow running on github actions using this CLI, so maybe it need to have env vars and command line arguments.
