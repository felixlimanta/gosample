Sample Go app - Auto Pull Staging 2

## Getting started

1. Clone this repo using `git clone https://github.com/tokopedia/gosample.git <your-project-name>`.

2. cd to <your-project-name> and delete the existing git repository by running `rm -rf .git`.

3. Initialize a new git repository with `git init`, `git add .` and `git commit -m "Initial commit"`.

4. Run `dep ensure` to install the dependencies. If you don't have [dep](https://github.com/golang/dep), install using homebrew etc.

5. Run `go build` and then ./<your-project-name> to start the local web server.

6. Go to `http://localhost:9000` and you should see the app running!
