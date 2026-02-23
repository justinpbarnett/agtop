# TODO

[ ] Accept images in new run input
[x] Allow select/cut/copy/paste and mouse select text in new run input
[ ] Move default model option of new run modal to be first in new run input
[x] ctrl+c should close the new run modal
[ ] instead of one hotkey per model type or workflow type in the new run modal, just have a hotkey for workflows and a hotkey for models. each otkey will cycle through all available options
[x] pressing y with detail panel selected should copy all contents of it
[ ] add a decorative loading spinner next to the agtop name just to reasure the user that the app is running and updating live
[x] details panel should show current step and current model and selected workflow for this run
[ ] pressing enter on a run in the runs panel should show a dropdown list of all concurrent runs happening or just one if one is the only run
[?] add ability to update (provide follow-on prompts) a run after it's completed
[x] accepting a completed run seems to do nothing. worktree may be getting pre-maturely cleaned up.
[x] format expanded log details. wrap markdown text
[x] in log panel, if json is a type: user and has a message: content, just show the content on the line. can show full json if expanded
[x] in log panel, hide successful rate limit queries
[x] in log panel, if message is a tool use, let's show that in a nice way instead of raw json
[x] in log panel, basically interpret all expected json responses into nice one-liners for the user to view, pressing enter to expand any of them should show the full, formatted, json response.
[x] drasically expand the default context deadlines
[x] update the default config to be agtop.toml and not a yaml file
