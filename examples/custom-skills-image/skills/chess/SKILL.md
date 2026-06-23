---
name: chess
description: Play a game of chess with the user. Use when the user asks to play chess, make a chess move, or asks about chess rules or notation.
license: MIT
compatibility: Requires no external tools
metadata:
  author: example
  version: "1.0"
allowed-tools: Read
---
# Chess

Use this skill when the user wants to play chess, make a move in an ongoing
game, or asks about chess rules and notation.

## When to use

- "Let's play a game of chess."
- "I'll open with e4."
- "What's algebraic notation?"
- Any message that looks like a chess move (e.g. "Nf3", "O-O", "Qxd5+").

## How to play

1. Ask the user which side they want (White or Black) if not already established.
2. Keep an internal 8×8 board state across turns — track piece positions,
   castling rights, en passant target, and whose move it is.
3. Accept moves in **Standard Algebraic Notation** (SAN): `e4`, `Nf3`,
   `O-O` (kingside castle), `Qxd5+` (capture with check), `e8=Q` (promotion).
   If the user writes in coordinate notation (`e2e4`) or descriptive notation,
   accept it but echo the SAN back so the conversation stays consistent.
4. After each user move, verify legality (piece movement, blocks, checks,
   castling through check, en passant timing). If illegal, explain why and
   ask for a different move.
5. Respond with your move plus a compact board diagram so the user can see
   the state. Use uppercase for White pieces, lowercase for Black, dots for
   empty squares, and rank/file labels:

   ```
     a b c d e f g h
   8 r . b q k b . r 8
   7 p p p . . p p p 7
   6 . . . . p . . . 6
   5 . . . . . . . . 5
   4 . . . . P . . . 4
   3 . . . . . . . . 3
   2 P P P P . P P P 2
   1 R . B Q K B . R 1
     a b c d e f g h
   ```

6. Announce checks, checkmates, and stalemates explicitly ("Check.",
   "Checkmate. White wins.", "Stalemate. Draw.").

## Notation reference

| Symbol | Meaning |
|--------|---------|
| `K Q R B N` | King, Queen, Rook, Bishop, Knight (uppercase = White) |
| `e4`, `Nf3`, `Bb5` | Pawn move / piece move |
| `x` | Capture (`Nxe5`) |
| `+` | Check (`Qd5+`) |
| `#` | Checkmate (`Qh7#`) |
| `O-O` | Kingside castle |
| `O-O-O` | Queenside castle |
| `=` | Promotion (`e8=Q`) |
| `e.p.` | En passant capture |

Read [rules](references/rules.md) for the full set of legal-move and
special-case rules if you're unsure about castling, en passant, or
promotion interactions.