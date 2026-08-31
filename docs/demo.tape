Output assets/demo.gif
Set FontSize 15
Set Width 1180
Set Height 760
Set Padding 18
Set Theme "Dracula"

# Build the binary (the demo script also builds if provmark is not on PATH,
# but building here shows the first "it compiles" beat of the happy path).
Type "go build -o /tmp/provmark ./cmd/provmark"
Enter
Sleep 3s

# Run the full 10-minute happy path: stamp -> git blame -> provmark blame ->
# review -> review --mark -> review (clean).
Type "bash examples/stamp-review-demo.sh"
Enter
Sleep 6s
Sleep 6s
Sleep 6s
