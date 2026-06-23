# Chess rules reference

Compact reference for the legal-move and special-case rules referenced by the
chess skill. This is the bundled deep-dive resource — the agent reads it on
demand via `skill://chess/references/rules.md` when a rule question arises
during play.

## Piece movement

| Piece | How it moves |
|-------|--------------|
| King | One square in any direction. |
| Queen | Any number of squares in any direction (rank, file, diagonal). |
| Rook | Any number of squares along rank or file. |
| Bishop | Any number of squares diagonally. |
| Knight | L-shape: (2,1) or (1,2) — the only piece that jumps over others. |
| Pawn | One square forward (two from its starting rank); captures one square diagonally forward. |

You cannot move through your own pieces (except the knight). You cannot move
onto a square occupied by your own piece.

## Special moves

### Castling

King moves two squares toward a rook; the rook jumps to the king's far side.
- **Kingside:** `O-O` (king from e1 → g1, rook from h1 → f1).
- **Queenside:** `O-O-O` (king from e1 → c1, rook from a1 → d1).

Conditions:
1. Neither the king nor that rook has moved before.
2. All squares between them are empty.
3. The king is not in check.
4. The king does not pass through or land on an attacked square.

### En passant

If your pawn is on its 5th rank (for Black, the 4th) and an enemy pawn moves
two squares forward to land beside yours, you may capture it as if it had
moved one square. The capture must be made **on the next move** or it lapses.

### Promotion

A pawn that reaches the last rank must promote to Queen, Rook, Bishop, or
Knight of the same color. Choose the piece explicitly: `e8=Q`, `e8=N` (a
"underpromotion" to a knight is legal and occasionally winning).

## Check and checkmate

- **Check:** the king is under direct attack. The player must respond by
  moving the king, blocking the attack, or capturing the attacker.
- **Checkmate:** the king is in check and has no legal escape. Game ends.
- **Stalemate:** the player to move has no legal moves but is **not** in
  check. The game is a draw.

## Draw rules

- **Insufficient material:** K vs K, K+N vs K, K+B vs K, K+B vs K+B (same
  color bishops). Draw.
- **Threefold repetition:** the same position (same side to move, same
  castling/en passant rights) appears three times. Claimable draw.
- **Fifty-move rule:** 50 moves by each player without a pawn move or a
  capture. Claimable draw.

## Naming convention reminder

This directory's frontmatter `name` is `chess`, so the bundled rules file is
read via the resource URI `skill://chess/references/rules.md`. Any other
non-`SKILL.md` file dropped here would also be auto-discovered as a
resource — the skill server discovers **all** files under the skill dir.