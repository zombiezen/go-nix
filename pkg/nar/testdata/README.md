# nar testdata

This is a collection of .nar files used for tests.

- `1byte-regular.nar` contains a regular file at the root with a single `\x01` byte.
- `empty-directory.nar` contains an empty directory at the root.
- `empty-file.nar` contains a regular empty file at the root.
- `hello-script.nar` contains a regular executable file at the root with a Hello, World shell script.
- `hello-world.nar` contains a regular file at the root with the content `Hello, World!\n`.
- `mini-drv.nar` contains three files: a.txt, bin/hello.sh, and hello.txt.
  a.txt contains `AAA\n`,
  bin/hello.sh contains a small shell script that cats hello.txt,
  and hello.txt contains `Hello, World!\n`.
- `symlink.nar` contains a symlink at the root with the target `/nix/store/somewhereelse`.
- `nar_1094wph9z4nwlgvsd53abfz8i117ykiv5dwnq9nnhz846s7xqd7d.nar` is a copy of the
  `/nix/store/00bgd045z0d4icpbc2yyz4gx48ak44la-net-tools-1.60_p20170221182432` derivation.

## Invalid Archives

- `invalid-order.nar` contains a directory with two subdirectories "b" and "a" (in that order).
  NAR directory entries are supposed to be ordered lexicographically.
- `only-magic.nar` contains the magic version header, but nothing else.
