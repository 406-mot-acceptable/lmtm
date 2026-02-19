# Tmux Setup

## Claude Agent Team Layout

Brain (team leader) top center, 2 workers flanking it, 3 workers across the bottom.

```
+----------+--------------------+----------+
|  worker  |                    |  worker  |
|          |      BRAIN         |          |
|          |  claude (leader)   |          |
+----------+--------------------+----------+
|  worker     |  worker     |  worker     |
+-------------+-------------+-------------+
```

### Quick Apply (current session)

The layout string is resolution-dependent. This one is for a 273x54 terminal:

```bash
tmux select-layout -t 0:0 'bb14,273x54,0,0[273x35,0,0{57x35,0,0,2,158x35,58,0,0,56x35,217,0,1},273x18,0,36{91x18,0,36,3,90x18,92,36,5,90x18,183,36,4}]'
```

### Script (works at any resolution)

Create 6 panes and apply the layout from scratch:

```bash
#!/bin/bash
# claude-team-layout.sh
# Creates 6 panes in the brain-center layout for claude agent teams.
# Run inside an existing tmux session.

SESSION="${1:-0}"
WINDOW="${2:-0}"
TARGET="$SESSION:$WINDOW"

# Kill existing panes except the first one
while [ "$(tmux list-panes -t "$TARGET" | wc -l)" -gt 1 ]; do
  tmux kill-pane -t "$TARGET.1"
done

# Create 5 more panes (6 total) - splits don't matter, layout overrides them
for i in $(seq 1 5); do
  tmux split-window -t "$TARGET"
  tmux select-layout -t "$TARGET" tiled  # prevent "no space" errors
done

# Apply tiled first, then swap brain into position
tmux select-layout -t "$TARGET" tiled

# Build and apply custom layout based on current terminal size
read COLS ROWS <<< "$(tmux display-message -t "$TARGET" -p '#{window_width} #{window_height}')"

# Top row gets ~65% of height, bottom row gets the rest
TOP_H=$(( (ROWS * 65 / 100) ))
BOT_H=$(( ROWS - TOP_H - 1 ))  # -1 for separator
BOT_Y=$(( TOP_H + 1 ))

# Top row: 3 columns - side workers ~20%, brain ~60%
SIDE_W=$(( COLS * 20 / 100 ))
BRAIN_W=$(( COLS - SIDE_W - SIDE_W - 2 ))  # -2 for separators
BRAIN_X=$(( SIDE_W + 1 ))
RIGHT_X=$(( BRAIN_X + BRAIN_W + 1 ))

# Bottom row: 3 equal columns
BOT_W1=$(( (COLS - 2) / 3 ))
BOT_W2=$BOT_W1
BOT_W3=$(( COLS - BOT_W1 - BOT_W2 - 2 ))
BOT_X2=$(( BOT_W1 + 1 ))
BOT_X3=$(( BOT_X2 + BOT_W2 + 1 ))

# Get pane IDs in current order
PANES=($(tmux list-panes -t "$TARGET" -F '#{pane_id}' | tr -d '%'))

# Layout string: top[{side,brain,side}, bottom{w,w,w}]
LAYOUT="${COLS}x${ROWS},0,0[${COLS}x${TOP_H},0,0{${SIDE_W}x${TOP_H},0,0,${PANES[0]},${BRAIN_W}x${TOP_H},${BRAIN_X},0,${PANES[1]},${SIDE_W}x${TOP_H},${RIGHT_X},0,${PANES[2]}},${COLS}x${BOT_H},0,${BOT_Y}{${BOT_W1}x${BOT_H},0,${BOT_Y},${PANES[3]},${BOT_W2}x${BOT_H},${BOT_X2},${BOT_Y},${PANES[4]},${BOT_W3}x${BOT_H},${BOT_X3},${BOT_Y},${PANES[5]}}]"

# Compute tmux checksum
CHECKSUM=$(python3 -c "
layout = '$LAYOUT'
csum = 0
for c in layout:
    csum = ((csum >> 1) + ((csum & 1) << 15) + ord(c)) & 0xffff
print(format(csum, '04x'))
")

tmux select-layout -t "$TARGET" "${CHECKSUM},${LAYOUT}"

echo "Layout applied. Swap the brain pane into position 1 (top center) if needed:"
echo "  tmux swap-pane -s <brain_pane_id> -t <center_pane_id>"
```

### Mouse Mode

Enabled in `~/.tmux.conf`:

```
set -g mouse on
```

Allows clicking panes to switch, dragging borders to resize, and scrolling.

### Useful Shortcuts

| Key | Action |
|-----|--------|
| `Ctrl+b Space` | Cycle through built-in layouts |
| `Ctrl+b z` | Zoom/unzoom current pane (fullscreen toggle) |
| `Ctrl+b q` | Show pane numbers briefly |
| `Ctrl+b ;` | Switch to last active pane |
