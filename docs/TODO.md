# TODO

[ ] Accept images in new run input
[x] Move default model option of new run modal to be first in new run input
[~] instead of one hotkey per model type or workflow type in the new run modal, just have a hotkey for workflows and a hotkey for models. each otkey will cycle through all available options
[ ] add a decorative loading spinner next to the agtop name just to reasure the user that the app is running and updating live. also add one to the bottom of the logs file while the run is going, maybe a dancing elipsis or something. also add a loading indicator to the run in the run panel if it's active. Basically add an indicator anytime we're still waiting for input or responses or stuff is still happening.
[ ] pressing enter on a run in the runs panel should show a dropdown list of all concurrent runs happening or just one if one is the only run
[?] add ability to update (provide follow-on prompts) a run after it's completed
[ ] Convert skills to output in json for easier reading by LLMs. Could also allow for things like the spec skill to specify if this should be broken down into multiple tasks and allow skipping the deconstruct skill and go straight to a parallel workflow?
[?] show full minutes and seconds in the detail panel of a run time, and hours, or even days if it gets to that. runs list can show the highest type (like 4m if it's actually 4m 13s)
[?] diff seems to be showing off main branch and not the current worktree or branch the run is working in
[ ] having a run selected moves the cost positioning around. selected runs are actually in the right alignment and everything else is off.
[ ] in log panel, shorten all file paths to be relative so they clutter the space less.
[ ] in log panel, don't put an expand arrow next to logs that don't have anything to expand.
[ ] current model being used is not showing properly in the details panel
[ ] the prompt in the details panel runs off the panel and is not wrapped. I want to be able to see the full prompt, but I also want to account for the fact that sometimes the prompts might be very large and i still want to see the rest of the details in the details panel. come up with a good solution for this problem
