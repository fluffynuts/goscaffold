goscaffold: a cli tool to scaffold projects for golang

Scenario 1: user wants to create a brand new project, from scratch

1. user would invoke the tool as such: `goscaffold <path to folder>`. The path can be relative or absolute.
2. goscaffold then takes the user through an interactive process:
   a. we need to create the repository at github using the `gh` tool. `gh` needs 3 pieces of information:
      - project name (text input, defaults to the leaf node of the provided folder name, ie `goscaffold path/to/some-project would use `some-project` as the default project name, and `goscaffold /path/to/another-project` would use `another-project` as the project name
      - organisation: use `gh org list` to provide a single select list for the user, but put their own account at the top (you can use `gh api user --jq .login` to get the user name. So if the user bobsaget belonged to some organisations, the list should be presented like so:
        Where should I create this repository?
        > bobsaget
          Organisation A
          Organisation B
      - public/private: use a single select list, default to public
      - after creating the github repo, check it out to the provided folder name
      - create a README.md with the project name at the top, underlined, eg:
        ```
        project-name
        ---
        ```
      - create a .gitignore file (you may use the .gitignore that is in this repository - perhaps embed it in the output binary)
  b. now, within that folder, using the provided vibe project as an example, I want the following set up:
    1. src folder with main.go, with a basic main routine that just prints "scaffolded with goscaffold"
    2. a Makefile that builds the project (and their companion make.sh and make.ps1 equivalents)
    3. a github workflow that builds, runs all tests, and produces release artifacts like vibe does
    4. pre-existing cli parameter support:
        --help should show help (all cli args, usage example)
        --version should print out the current version of the app, in the format "app-name <version> (<git sha> <build date>)", eg vibe reports: "vibe 0.1.37 (5da629aa28c2, built 2026-10-07T06:38:08Z)"
        --install should copy the binary to ~/.local/bin and check that that folder is in the user's PATH. If it isn't, then on windows, add it to the path and warn the user that they need to restart the terminal to find it in their path. On other operating systems, simply warn that the folder is not in the user's path (there are many ways for the user to remedy this on linux, at least)
        --upgrade should download the latest release artifact from the releases page for the current machine, unpack it in a temp folder, and run --install on it
      this should be built as a separate module from main.go, and imported into main.go (vibe has it all embedded in main.go - but I want to be able to extend an existing project later)
 

If the folder already exists, but it's empty, proceed as above. If the folder doesn't exist, create it and proceed as above.


Scenario 2: user wants to scaffold an existing project
1. if the project is already version-controlled with git, skip the repo creation step
2. if go sources already exist, still create the module for cli argument handling (--help, --version, --install, --upgrade) and warn the user that they will need to use it to support --install and --upgrade
3. create the build scripts (Makefile, make.sh, make.ps1)
4. create the github workflow which builds, tests and produces release artifacts

When creating files, if the target file already exists, prompt the user with something like:
"<file> already exists - replace it? [y/N]" (default no, so if they press enter, their files are left alone)

