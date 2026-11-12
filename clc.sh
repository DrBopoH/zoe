#!/usr/bin/env bash
# zoe.sh — мост между репозиторием и чатом с Claude.
# Положи в корень репозитория, chmod +x zoe.sh, один раз: ./zoe.sh init. Справка: ./zoe.sh help
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || { echo "✗ Запусти внутри git-репозитория" >&2; exit 1; }
cd "$ROOT"

DL=${ZOE_DL:-$HOME/Downloads}
mkdir -p "$DL"
DL=$(cd "$DL" && pwd)

BE=backend
FE=frontend
CL=.claude                  # рабочий файл Claude (в .gitignore)
CM=.commits                 # служебная папка (в .gitignore)
AP=$CM/applied              # применённые патчи + снимки «до»
PU=$CM/pushed               # пакеты, отправленные в чат
KEEP_PUSHED=5
M_AUTO='<!-- zoe:auto -->'
M_RUNS='<!-- zoe:runs -->'
CONV='^(feat|fix|refactor|perf|test|docs|style|chore)(!)?: .{1,72}$'
SECRET='\.(db|sqlite|sqlite3|env|pem|key)(-wal|-shm)?$'
SKIP='\.(db|sqlite|sqlite3|env|pem|key)(-wal|-shm)?$|\.(csv|pdf|zip|7z)$|(^|/)(node_modules|dist|\.git)/'
CHECK=0
QUIET=0
SELF=$(basename "$0")
SZ=$(command -v 7zz || command -v 7z || command -v 7za || true)
[[ -n $SZ ]] || { echo "✗ нужен 7z: sudo pacman -S 7zip" >&2; exit 1; }

die() { echo "✗ $*" >&2; exit 1; }
cur_branch() { git symbolic-ref --short HEAD 2>/dev/null || echo "(нет коммитов)"; }
sec() { printf '\n## %s\n\n```\n' "$1"; }
endsec() { printf '```\n'; }
stamp() { date +%Y%m%d-%H%M%S; }

# следующий номер записи 0001, 0002, … в папке
next_num() {
  local m
  m=$(ls -1 "$1" 2>/dev/null | sed -n 's/^\([0-9]\{4\}\)-.*/\1/p' | sort -n | tail -1 || true)
  printf '%04d' $(( 10#${m:-0} + 1 ))
}

# файлы проекта (без баз, ключей, csv/pdf, node_modules) в заданных папках
project_files() {
  git ls-files -co --exclude-standard -- "$@" | sort -u | { grep -Ev "$SKIP" || true; } | while IFS= read -r f; do
    if [[ -f $f ]]; then printf '%s\n' "$f"; fi
  done
}

# ---- архивы через 7z (формат остаётся zip, чтобы их можно было читать где угодно)
# arc_create <архив>  — имена файлов со stdin
arc_create() {
  local lf; lf=$(mktemp)
  { grep -v '^$' || true; } > "$lf"
  if [[ -s $lf ]]; then rm -f "$1"; "$SZ" a -tzip -y -spd -bso0 -bsp0 "$1" "@$lf" >/dev/null; fi
  rm -f "$lf"
}
# arc_names <архив> — пути внутри архива, у папок на конце /
arc_names() {
  "$SZ" l -slt -ba "$1" | awk -v RS= -F'\n' '{p="";d=0; for(i=1;i<=NF;i++){ if($i ~ /^Path = /){p=substr($i,8)} if($i ~ /^Folder = \+/){d=1} } if(p!="") print p (d?"/":"")}'
}
# arc_extract <архив> <папка> — с перезаписью
arc_extract() { "$SZ" x -y -aoa -bso0 -bsp0 -o"$2" "$1" '-x!__MACOSX' >/dev/null; }

# снимок папок backend/ и frontend/ в zip
snapshot_to() {
  local zip=$1 list
  list=$(project_files "$BE" "$FE")
  rm -f "$zip"
  printf '%s\n' "$list" | arc_create "$zip"
}

# Копирует в буфер сам файл (для вставки в чат Ctrl+V), а не строку с путём.
# Режим: ZOE_CLIP=uri (по умолчанию, text/uri-list) | gnome (x-special/gnome-copied-files) | path (просто текст пути)
clip() {
  local abs uri mode=${ZOE_CLIP:-uri} type data
  abs=$(realpath "$1"); uri="file://${abs// /%20}"
  case "$mode" in
    uri)   type=text/uri-list;                    data=$(printf '%s\n' "$uri") ;;
    gnome) type=x-special/gnome-copied-files;     data=$(printf 'copy\n%s' "$uri") ;;
    path)  type=text/plain;                       data=$abs ;;
    *) return 1 ;;
  esac
  if command -v wl-copy >/dev/null 2>&1 && printf '%s' "$data" | wl-copy -t "$type" 2>/dev/null; then return 0; fi
  if command -v xclip >/dev/null 2>&1 && printf '%s' "$data" | xclip -selection clipboard -t "$type" 2>/dev/null; then return 0; fi
  return 1
}

warn_ignored() {
  local p
  for p in "$CL" "$CM/"; do
    git check-ignore -q "$p" 2>/dev/null || echo "⚠ $p не в .gitignore — добавь (или ./${SELF} init)"
  done
}

tree_view() {
  if command -v eza >/dev/null 2>&1; then eza --tree --level=4 -I 'node_modules|.git|dist|.commits'
  elif command -v tree >/dev/null 2>&1; then tree -L 4 -I 'node_modules|.git|dist|.commits'
  else git ls-files | head -300; fi
}

# ------------------------------------------------ .claude: заметки / снимок / вывод команд
# Структура файла:
#   (всё до маркера) — заметки Claude, скрипт их не трогает
#   <!-- zoe:auto -->  — снимок репозитория, перезаписывается при look/push
#   <!-- zoe:runs -->  — вывод команд (./zoe.sh run …), очищается после push
stub_notes() { printf '# .claude — рабочий файл Claude (не коммитится)\n\n(заметки пока пусты)\n\n'; }
notes_part() { if [[ -f $CL ]] && grep -qF "$M_AUTO" "$CL"; then awk -v m="$M_AUTO" 'index($0,m){exit} {print}' "$CL"; elif [[ -f $CL ]]; then cat "$CL"; echo; else stub_notes; fi; }
runs_part() { if [[ -f $CL ]]; then awk -v m="$M_RUNS" 'f{print} index($0,m){f=1}' "$CL"; fi; }

snapshot_md() {
  echo "## Снимок репозитория — $(date '+%F %T')"
  sec "Ветка и статус"; git status -sb; endsec
  sec "История и ветки"
  git log --oneline --decorate -n 15 2>/dev/null || echo "(коммитов нет)"
  echo; git branch -vv 2>/dev/null || true
  git stash list
  endsec
  sec "Изменения относительно HEAD"; git diff --stat HEAD 2>/dev/null || true; endsec
  sec "Дерево"; tree_view; endsec
  sec "Инструменты"
  for c in "go version" "bun --version" "node --version"; do $c 2>/dev/null || echo "(нет: ${c%% *})"; done
  if [[ -f $BE/go.mod ]]; then head -3 "$BE/go.mod"; fi
  if [[ -f $FE/package.json ]]; then grep -A8 '"scripts"' "$FE/package.json" || true; fi
  endsec
  sec "Маршруты бэкенда (main.go)"
  if [[ -f $BE/main.go ]]; then grep -nE 'Handle(Func)?\(' "$BE/main.go" || echo "(не найдено)"; fi
  endsec
  sec "Базы (только размеры, содержимое не читается)"
  find . -name '*.db' -not -path '*/node_modules/*' -exec ls -la {} \; 2>/dev/null || true
  endsec
  sec "TODO / FIXME"; { git grep -nE 'TODO|FIXME' -- ':!*.md' ":!$SELF" ":!$CL" 2>/dev/null | head -30; } || true; endsec
  if (( CHECK )); then
    sec "go test"; (cd "$BE" && go test ./... 2>&1 | tail -20) || true; endsec
    sec "сборка фронта"; (cd "$FE" && bun run build 2>&1 | tail -15) || true; endsec
  fi
}

# пересобрать .claude: заметки и вывод команд сохраняются, снимок обновляется
write_claude() {
  local notes runs tmp
  notes=$(mktemp); runs=$(mktemp); tmp=$(mktemp)
  notes_part > "$notes"; runs_part > "$runs"
  { cat "$notes"; echo "$M_AUTO"; snapshot_md; echo; echo "$M_RUNS"; cat "$runs"; } > "$tmp"
  mv "$tmp" "$CL"
  rm -f "$notes" "$runs"
}

# ------------------------------------------------ init
cmd_init() {
  mkdir -p "$AP" "$PU"
  local p
  for p in "$CL" "$CM/"; do
    if ! git check-ignore -q "$p" 2>/dev/null; then
      printf '%s\n' "$p" >> .gitignore
      echo "+ .gitignore: $p"
    fi
  done
  [[ -f $CL ]] || { stub_notes > "$CL"; echo "+ $CL"; }
  echo "✓ готово: $AP, $PU"
}

# ------------------------------------------------ look
cmd_look() {
  [[ "${1:-}" == "--check" ]] && CHECK=1
  write_claude
  echo "✓ $CL обновлён"
  warn_ignored
  local n; n=$(git status --porcelain --untracked-files=no | wc -l)
  if (( n > 0 )); then echo "⚠ незакоммиченных файлов: $n — закоммить перед следующей фичей"; fi
  return 0
}

# ------------------------------------------------ run
# ./zoe.sh run 'cd backend && go test ./...'   — вывод попадёт в .claude (и на экран)
cmd_run() {
  (( $# > 0 )) || die "укажи команду: ./${SELF} run 'go test ./...'"
  if ! { [[ -f $CL ]] && grep -qF "$M_RUNS" "$CL"; }; then write_claude; fi
  local out rc=0
  out=$(bash -c "$*" 2>&1) || rc=$?
  printf '%s\n' "$out" | tail -n 200
  { printf '\n### `%s` (%s, код %s)\n\n```\n' "$*" "$(date '+%F %T')" "$rc"; printf '%s\n' "$out" | tail -n 200; printf '```\n'; } >> "$CL"
  echo "→ записано в $CL (код $rc)"
  return "$rc"
}

# ------------------------------------------------ push
# Пакет для чата → .commits/pushed/NNNN-….zip, путь в буфер обмена.
# push — всё (backend/ + frontend/ + .claude); можно выбрать части: push be | push fe | push note | push be note …
cmd_push() {
  local want_be=0 want_fe=0 want_note=0 any=0 a
  for a in "$@"; do
    case "$a" in
      be) want_be=1; any=1 ;;
      fe) want_fe=1; any=1 ;;
      note|claude) want_note=1; any=1 ;;
      --check) CHECK=1 ;;
      *) die "push [be] [fe] [note] [--check] — без аргументов: всё" ;;
    esac
  done
  if (( !any )); then want_be=1; want_fe=1; want_note=1; fi
  mkdir -p "$PU"
  write_claude

  local dirs=() tags=() tag files=""
  if (( want_be )); then dirs+=("$BE"); tags+=(be); fi
  if (( want_fe )); then dirs+=("$FE"); tags+=(fe); fi
  if (( want_note )); then tags+=(note); fi
  if (( ${#tags[@]} == 3 )); then tag=all; else tag=$(IFS=+; echo "${tags[*]}"); fi

  local zip="$PU/$(next_num "$PU")-$(stamp)-$tag.zip"
  if (( ${#dirs[@]} > 0 )); then files=$(project_files "${dirs[@]}"); fi
  if (( want_note )); then files+=$'\n'"$CL"; fi
  printf '%s\n' "$files" | arc_create "$zip"

  # вывод команд отправлен вместе с .claude — очищаем
  if (( want_note )); then
    local tmp; tmp=$(mktemp)
    awk -v m="$M_RUNS" '{print} index($0,m){exit}' "$CL" > "$tmp" && mv "$tmp" "$CL"
  fi
  # храним последние $KEEP_PUSHED пакетов
  ls -1t "$PU"/*.zip 2>/dev/null | tail -n +$((KEEP_PUSHED + 1)) | while IFS= read -r f; do rm -f "$f"; done
  echo "✓ $zip ($(du -h "$zip" | cut -f1), файлов: $(arc_names "$zip" | grep -vc '/$' || true))"
  if clip "$zip"; then echo "📋 в буфере (${ZOE_CLIP:-uri}): вставь Ctrl+V в чат"; else echo "(буфер недоступен: поставь wl-clipboard или xclip)"; fi
  warn_ignored
}

# ------------------------------------------------ apply
# Забирает из Downloads ВСЕ patch*.zip (пути backend/… frontend/… [.claude] от корня) от старых к новым
# и применяет по очереди, затем backend.zip / frontend.zip (пути от своей папки), если есть.
# Для каждого патча: снимок before.zip → применение → архив ПЕРЕНОСИТСЯ (mv) в .commits/applied/NNNN-…/.
# Downloads остаётся чистым.
cmd_apply() {
  local force=0
  [[ "${1:-}" == "--force" ]] && force=1
  local items=() p
  # все patch*.zip, от старых к новым (по времени файла): применяются последовательно, поздние перекрывают ранние
  while IFS= read -r p; do
    if [[ -n $p ]]; then items+=("$p|."); fi
  done < <(ls -1tr "$DL"/patch*.zip 2>/dev/null || true)
  if [[ -f "$DL/backend.zip" ]]; then items+=("$DL/backend.zip|$BE"); fi
  if [[ -f "$DL/frontend.zip" ]]; then items+=("$DL/frontend.zip|$FE"); fi
  (( ${#items[@]} > 0 )) || die "в $DL нет patch.zip / backend.zip / frontend.zip"

  if (( !force )) && [[ -n "$(git status --porcelain --untracked-files=no)" ]]; then
    git status -s --untracked-files=no
    die "дерево не чистое. Сначала ./${SELF} commit …, либо ./${SELF} apply --force"
  fi

  local it zip dir bad
  for it in "${items[@]}"; do
    zip=${it%%|*}
    bad=$(arc_names "$zip" | grep -E '^/|(^|/)\.\.(/|$)' || true)
    [[ -z $bad ]] || die "$(basename "$zip") содержит небезопасные пути: $bad"
  done

  local touched=() e prefix entry
  for it in "${items[@]}"; do
    zip=${it%%|*}; dir=${it##*|}
    # у каждого патча своя запись в applied со своим снимком «до» — откатываться можно по одному
    entry="$AP/$(next_num "$AP")-$(stamp)"
    mkdir -p "$entry"
    snapshot_to "$entry/before.zip"
    echo "● $(basename "$entry"): снимок «до» сохранён"
    prefix=""; if [[ $dir != . ]]; then prefix="$dir/"; fi
    echo "── $(basename "$zip") → ${dir}/"
    while IFS= read -r e; do
      if [[ $e == */ || $e == __MACOSX/* ]]; then continue; fi
      if [[ -e "$prefix$e" ]]; then echo "  ~ $prefix$e"; else echo "  + $prefix$e"; fi
      case "$prefix$e" in
        $BE/*) touched+=(be) ;;
        $FE/*) touched+=(fe) ;;
        $CL|$CM/*) ;;
        *) touched+=(docs) ;;
      esac
    done < <(arc_names "$zip")
    mkdir -p "$dir"
    arc_extract "$zip" "$dir"
    mv "$zip" "$entry/$(basename "$zip")"
  done

  echo; git status -s --untracked-files=all | head -40
  if (( QUIET )); then return 0; fi
  echo; echo "Дальше:"
  local br; br=$(cur_branch)
  if [[ $br == main || $br =~ ^(dev|build|release)$ ]]; then
    echo "  ./${SELF} branch <слово>      # ты на $br — перейди в wip-ветку, правки перенесутся"
  fi
  local s
  for s in be fe docs; do
    if printf '%s\n' "${touched[@]:-}" | grep -qx "$s"; then echo "  ./${SELF} commit $s \"fix: …\""; fi
  done
  echo "  ./${SELF} rollback            # если что-то пошло не так"
}

# ------------------------------------------------ rollback
# ./zoe.sh rollback list | rollback [N]
# Перезаписывает backend/ и frontend/ снимком «до» записи N (по умолчанию — последней).
# Перед этим сам делает снимок текущего состояния, поэтому rollback можно откатить ещё одним rollback.
cmd_rollback() {
  local arg=${1:-last} d
  if [[ $arg == list ]]; then
    if ! ls -1d "$AP"/*/ >/dev/null 2>&1; then echo "(записей нет)"; return 0; fi
    for d in $(ls -1d "$AP"/*/ | sort); do
      d=${d%/}
      echo "$(basename "$d")  [$(ls -1 "$d" | grep -v '^before.zip$' | tr '\n' ' ' || true)]"
    done
    return 0
  fi
  local dir
  if [[ $arg == last ]]; then
    dir=$(ls -1d "$AP"/*/ 2>/dev/null | sort | tail -1 || true)
  else
    [[ $arg =~ ^[0-9]+$ ]] || die "rollback list | rollback [номер]"
    dir=$(ls -1d "$AP"/"$(printf '%04d' "$((10#$arg))")"-*/ 2>/dev/null | head -1 || true)
  fi
  [[ -n ${dir:-} ]] || die "нет такой записи (./${SELF} rollback list)"
  dir=${dir%/}

  local entry="$AP/$(next_num "$AP")-$(stamp)-rollback"
  mkdir -p "$entry"
  snapshot_to "$entry/before.zip"
  echo "● текущее состояние сохранено: $entry/before.zip (откат отката: ./${SELF} rollback)"

  local f
  project_files "$BE" "$FE" | while IFS= read -r f; do rm -f -- "$f"; done
  find "$BE" "$FE" -depth -type d -empty -not -path '*/node_modules*' -delete 2>/dev/null || true
  if [[ -f $dir/before.zip ]]; then arc_extract "$dir/before.zip" .; fi
  echo "✓ backend/ и frontend/ восстановлены до состояния перед $(basename "$dir")"
  git status -s | head -20
}

# ------------------------------------------------ branch
# ---- именованные коммиты (заморозка фичи): «сводка | vR.B.C+T.<полный хэш замороженного коммита>»
NAMED_RE='\| v[0-9A-Za-z.]+\.[0-9]+\+[0-9]+\.[0-9a-f]{7,40}$'
VER_RE='v([0-9A-Za-z.]+)\.([0-9]+)\+([0-9]+)\.[0-9a-f]{7,40}$'
is_named() { [[ $(git log -1 --format=%s "$1" 2>/dev/null) =~ $NAMED_RE ]]; }
last_named() { git log -E --grep="$NAMED_RE" -1 --format=%H "${1:-HEAD}" 2>/dev/null || true; }

# от чего начинать новую ветку: свежий dev (после PR он содержит именованные коммиты), иначе последний именованный коммит, иначе HEAD
base_ref() {
  if git rev-parse -q --verify refs/remotes/origin/dev >/dev/null; then echo origin/dev
  elif git rev-parse -q --verify refs/heads/dev >/dev/null; then echo dev
  else
    local n; n=$(last_named HEAD)
    echo "${n:-HEAD}"
  fi
}

cmd_branch() {
  local w="${1:-}"
  [[ $w =~ ^[a-z0-9-]+$ ]] || die "слово: латиница строчными, цифры, дефис. Пример: ./${SELF} branch icu-ledger"
  local b; b=$(base_ref)
  git switch -c "wip/$(date +%F)-$w" "$b"
  echo "● ветка $(cur_branch) ← $b"
}

# ./clc.sh next <слово> — новая тема после PR: git fetch, ветка от свежего dev (или последнего именованного коммита)
cmd_next() {
  [[ -z $(git status --porcelain --untracked-files=no) ]] || die "есть незакоммиченные правки: закоммить или ./${SELF} park"
  git fetch -q origin 2>/dev/null || echo "(fetch не удался — беру локальное состояние)"
  if git rev-parse -q --verify refs/heads/dev >/dev/null && git rev-parse -q --verify refs/remotes/origin/dev >/dev/null; then
    git branch -f dev origin/dev 2>/dev/null || true
  fi
  cmd_branch "${1:-}"
}

# ------------------------------------------------ git-инструменты
is_protected() { [[ $1 =~ ^(dev|build|release)$ ]]; }
pushed_head() { git branch -r --contains HEAD 2>/dev/null | grep -q .; }
has_patches() { compgen -G "$DL/patch*.zip" >/dev/null || [[ -f $DL/backend.zip || -f $DL/frontend.zip ]]; }

# ensure_wip [слово] — гарантирует рабочую ветку: wip/* остаётся, иначе создаётся wip/<дата>-<слово>
ensure_wip() {
  local w=${1:-} br; br=$(cur_branch)
  if [[ $br == wip/* ]]; then return 0; fi
  if [[ -n $w ]]; then
    [[ $w =~ ^[a-z0-9-]+$ ]] || die "слово ветки: латиница строчными, цифры, дефис"
    local b; b=$(base_ref)
    git switch -qc "wip/$(date +%F)-$w" "$b"
    echo "● ветка $(cur_branch) ← $b"
    return 0
  fi
  if is_protected "$br"; then die "в $br коммитит только автоматика. Добавь -b <слово>"; fi
  if [[ $br == main ]]; then echo "⚠ ты на main. Лучше добавить -b <слово>"; fi
}

# после коммита напомнить, если в других папках остались незакоммиченные правки
warn_leftover() {
  local n; n=$(git status --porcelain --untracked-files=all | wc -l)
  if (( n > 0 )); then
    echo "⚠ не закоммичено файлов: $n (другой scope). Смотри: ./${SELF} look"
    git status -s | head -8
  fi
}

# stage_scope <be|fe|docs|all> — в индекс + защита от баз и ключей
stage_scope() {
  case "$1" in
    be) git add -A -- "$BE" ;;
    fe) git add -A -- "$FE" ;;
    docs) git add -A -- '*.md' ;;
    all) git add -A ;;
    *) die "scope: be | fe | docs | all" ;;
  esac
  if git diff --cached --name-only | grep -Eq "$SECRET"; then
    git diff --cached --name-only | grep -E "$SECRET" >&2 || true
    git reset -q
    die "в коммит попали базы/ключи — всё снято со стейджа. Добавь их в .gitignore"
  fi
}

# run_checks <scope> — go test / сборка фронта; вывод пишется в .claude
run_checks() {
  local rc=0
  case "$1" in be|all) cmd_run "cd $BE && go test ./..." || rc=1 ;; esac
  case "$1" in fe|all) cmd_run "cd $FE && bun run build" || rc=1 ;; esac
  return "$rc"
}

check_msg() { [[ $1 =~ $CONV ]] || die "сообщение вида 'feat: add x' (feat|fix|refactor|perf|test|docs|style|chore), до 72 символов после типа"; }

# ./clc.sh commit <be|fe|docs|all> "тип: что сделано" [-n] [-b слово]
#   проверки (go test / сборка) → индекс → коммит; -n без проверок; -b создаёт wip-ветку, если ты не в ней
cmd_commit() {
  local nocheck=0 bword="" args=()
  while (( $# )); do
    case "$1" in -n) nocheck=1 ;; -b) shift; bword=${1:-} ;; *) args+=("$1") ;; esac
    shift
  done
  local scope="${args[0]:-}" msg="${args[1]:-}"
  [[ -n $scope && -n $msg ]] || die "формат: ./${SELF} commit <be|fe|docs|all> \"тип: что сделано\" [-n] [-b слово]"
  check_msg "$msg"
  case "$scope" in be|fe|docs|all) ;; *) die "scope: be | fe | docs | all" ;; esac
  ensure_wip "$bword"
  if (( !nocheck )); then run_checks "$scope" || die "проверки не прошли, коммита нет. Лог в .claude (./${SELF} push note)"; fi
  stage_scope "$scope"
  git diff --cached --quiet && die "в $scope нечего коммитить"
  git commit -q -m "$msg"
  git log --oneline -n 1
  warn_leftover
}

# ./clc.sh ship <be|fe|docs|all> "тип: что сделано" [-b слово] [-p] [-f] [-n] [-z "сводка фичи" [-v R.B]]
#   -z: этим коммитом фича закончена — сразу влить ветку в dev одним именованным коммитом (см. land)
#   всё одной строкой: (ветка) → apply всех патчей из Downloads → проверки → коммит → (-p) push пакета в буфер
#   -f apply --force, -n без проверок
cmd_ship() {
  local nocheck=0 bword="" dopush=0 force=0 zsum="" zver="" args=()
  while (( $# )); do
    case "$1" in -n) nocheck=1 ;; -b) shift; bword=${1:-} ;; -p) dopush=1 ;; -f) force=1 ;; -z) shift; zsum=${1:-} ;; -v) shift; zver=${1:-} ;; *) args+=("$1") ;; esac
    shift
  done
  local scope="${args[0]:-}" msg="${args[1]:-}"
  [[ -n $scope && -n $msg ]] || die "формат: ./${SELF} ship <be|fe|docs|all> \"тип: что сделано\" [-b слово] [-p] [-f] [-n]"
  check_msg "$msg"
  case "$scope" in be|fe|docs|all) ;; *) die "scope: be | fe | docs | all" ;; esac
  ensure_wip "$bword"
  if has_patches; then
    QUIET=1
    if (( force )); then cmd_apply --force; else cmd_apply; fi
    QUIET=0
  else
    echo "(патчей в Downloads нет — apply пропущен)"
  fi
  if (( !nocheck )); then
    run_checks "$scope" || die "проверки не прошли, коммита нет. Лог в .claude. Дальше: ./${SELF} push note (мне) или ./${SELF} rollback"
  fi
  stage_scope "$scope"
  git diff --cached --quiet && die "в $scope нечего коммитить"
  git commit -q -m "$msg"
  git log --oneline -n 1
  if [[ -n $zsum ]]; then if [[ -n $zver ]]; then cmd_land "$zsum" -v "$zver"; else cmd_land "$zsum"; fi; fi
  warn_leftover
  if (( dopush )); then cmd_push; fi
}

# next_version [R.B] [rev] — следующая строка версии «vR.B.C+T» по последнему именованному коммиту в rev (по умолчанию HEAD).
# Другая сборка B (-v) обнуляет C, T идёт дальше. Первое вливание в репо — только с -v.
next_version() {
  local want=${1:-} n subj base="" c=0 t=0
  n=$(last_named "${2:-HEAD}")
  if [[ -n $n ]]; then
    subj=$(git log -1 --format=%s "$n")
    if [[ $subj =~ $VER_RE ]]; then base=${BASH_REMATCH[1]}; c=${BASH_REMATCH[2]}; t=${BASH_REMATCH[3]}; fi
  fi
  if [[ -n $want && $want != "$base" ]]; then base=$want; c=0; fi
  [[ -n $base ]] || die "первое вливание в репо: добавь -v R.B (например -v 1.a1)"
  echo "v${base}.$((c + 1))+$((t + 1))"
}

# land_one <родитель> <коммит-фичи> "сводка" [R.B] — сжатый именованный коммит: дерево коммита-фичи, родитель = текущий dev,
# сообщение «сводка | vR.B.C+T.<короткий хэш (7) последнего атомарного коммита фичи>». Печатает хэш нового коммита.
land_one() {
  local parent=$1 feat=$2 sum=$3 ver=${4:-} v
  v=$(next_version "$ver" "$parent")
  git commit-tree "$feat^{tree}" -p "$parent" -m "$sum | $v.$(git rev-parse --short=7 "$feat")"
}

# ./clc.sh land "сводка фичи" [-v R.B]            — вливание текущей wip-ветки в dev ОДНИМ именованным коммитом (аналог PR → Squash and merge)
# ./clc.sh land --plan <файл> [-v R.B]            — то же для готовой истории: по строке «<хэш> сводка» на фичу (строки «f …» и «#» пропускаются)
#   Атомарные коммиты и wip-ветка остаются локально как есть; dev получает только именованные коммиты. Файлы в рабочем дереве не трогаются.
#   Когда у проекта появится remote, dev меняется только через PR на GitHub — land тогда отказывается.
cmd_land() {
  local ver="" plan="" args=()
  while (( $# )); do case "$1" in -v) shift; ver=${1:-} ;; --plan) shift; plan=${1:-} ;; *) args+=("$1") ;; esac; shift; done
  if git remote get-url origin >/dev/null 2>&1; then die "есть remote: dev меняется только через PR (Squash and merge, в заголовке — версия)"; fi
  [[ $(cur_branch) == wip/* ]] || die "только из wip-ветки"
  # план может лежать в Downloads как patch.zip (.commits/regroup.txt) — забираем его тем же apply, что и ship
  if [[ -n $plan ]] && has_patches; then QUIET=1; cmd_apply --force; QUIET=0; fi
  local tip="" base
  if git rev-parse -q --verify refs/heads/dev >/dev/null; then
    tip=$(git rev-parse dev)
    git merge-base --is-ancestor "$tip" HEAD || die "ветка не от текущего dev — новые темы начинай через ./${SELF} next <слово>"
  else
    base=$(git merge-base HEAD main 2>/dev/null) || die "нет main — не от чего начинать dev"
    tip=$base
  fi
  local old=""; git rev-parse -q --verify refs/heads/dev >/dev/null && old=$(git rev-parse dev)
  if [[ -n $plan ]]; then
    [[ -f $plan ]] || die "нет файла $plan"
    local line h rest n=0
    while IFS= read -r line; do
      [[ -z $line || $line == \#* || $line =~ ^f[[:space:]] ]] && continue
      h=${line%% *}; rest=${line#* }; [[ $h != "$line" ]] || die "строка без сводки: $line"
      git cat-file -e "$h^{commit}" 2>/dev/null || die "нет коммита $h"
      git merge-base --is-ancestor "$h" HEAD || die "$h не в этой ветке"
      tip=$(land_one "$tip" "$h" "$rest" "$ver"); n=$((n + 1))
    done < "$plan"
    (( n > 0 )) || die "в плане нет строк «<хэш> сводка»"
  else
    local sum="${args[0]:-}"
    [[ -n $sum ]] || die "формат: ./${SELF} land \"сводка фичи\" [-v R.B]"
    [[ $(git rev-parse "HEAD^{tree}") != $(git rev-parse "$tip^{tree}") ]] || die "в ветке нет изменений относительно dev"
    tip=$(land_one "$tip" HEAD "$sum" "$ver")
  fi
  if [[ -n $old ]]; then git update-ref refs/heads/dev "$tip" "$old"; else git update-ref refs/heads/dev "$tip"; fi
  echo "✓ dev:"; git log --oneline -n "${n:-1}" dev
  echo "Дальше: ./${SELF} next <слово> — новая тема от dev"
}

# ./clc.sh amend [be|fe|docs|all] ["новое сообщение"] — добавить правки в последний коммит (если он ещё не на remote)
cmd_amend() {
  local scope=${1:-all} msg=${2:-}
  is_protected "$(cur_branch)" && die "в $(cur_branch) нельзя"
  if pushed_head; then die "последний коммит уже на remote — amend запрещён"; fi
  stage_scope "$scope"
  if [[ -n $msg ]]; then check_msg "$msg"; git commit -q --amend -m "$msg"; else git commit -q --amend --no-edit; fi
  git log --oneline -n 1
}

# ./clc.sh undo — снять последний коммит, правки останутся в индексе (если коммит ещё не на remote)
cmd_undo() {
  is_protected "$(cur_branch)" && die "в $(cur_branch) нельзя"
  if pushed_head; then die "последний коммит уже на remote"; fi
  git log --oneline -n 1
  git reset -q --soft HEAD~1
  echo "✓ коммит снят, правки в индексе"
}

# ./clc.sh park [имя] | park list | unpark — спрятать/вернуть незакоммиченное (stash с untracked)
cmd_park() {
  if [[ "${1:-}" == list ]]; then git stash list; return 0; fi
  git stash push -u -m "${SELF} $(date '+%F %T') ${1:-}"
  echo "✓ спрятано. Вернуть: ./${SELF} unpark"
}
cmd_unpark() { git stash pop; }

# ./clc.sh good [заметка] | good list — проверки (тесты + сборка) → метка good/<время> на чистом коммите
cmd_good() {
  if [[ "${1:-}" == list ]]; then git tag -l 'good/*' -n1 --sort=-creatordate | head -20; return 0; fi
  [[ -z "$(git status --porcelain --untracked-files=no)" ]] || die "дерево не чистое — сначала commit"
  run_checks all || die "проверки не прошли — метку не ставлю"
  local tag="good/$(stamp)"
  git tag -a "$tag" -m "${1:-good}"
  echo "✓ $tag на $(git rev-parse --short HEAD) — рабочая версия. Вернуться: ./${SELF} restore"
}

# ./clc.sh restore [метка|коммит] — перезаписать backend/ и frontend/ состоянием метки (по умолчанию последняя good)
# Перед этим снимок текущего состояния в .commits/applied/…-restore/ (откат: rollback)
cmd_restore() {
  local ref=${1:-}
  if [[ -z $ref ]]; then ref=$(git tag -l 'good/*' --sort=-creatordate | head -1 || true); fi
  [[ -n $ref ]] || die "нет меток good — сначала ./${SELF} good"
  git rev-parse -q --verify "$ref^{commit}" >/dev/null || die "нет такого ref: $ref"
  local entry="$AP/$(next_num "$AP")-$(stamp)-restore"
  mkdir -p "$entry"; snapshot_to "$entry/before.zip"
  echo "● текущее состояние сохранено: $entry/before.zip (вернуть: ./${SELF} rollback)"
  local f
  project_files "$BE" "$FE" | while IFS= read -r f; do rm -f -- "$f"; done
  find "$BE" "$FE" -depth -type d -empty -not -path '*/node_modules*' -delete 2>/dev/null || true
  git checkout "$ref" -- "$BE" "$FE"
  git reset -q -- "$BE" "$FE"
  echo "✓ backend/ и frontend/ = $ref (в рабочей папке, не закоммичено)"
  git status -s | head -20
}

# ./clc.sh publish — запушить текущую wip-ветку на origin (для PR в dev, Rebase and merge)
cmd_publish() {
  local br; br=$(cur_branch)
  if is_protected "$br" || [[ $br == main ]]; then die "из $br не публикуем: только wip-ветки"; fi
  git push -u origin "$br"
  echo "→ дальше PR в dev (Rebase and merge)"
}

# ./clc.sh sw <ветка> — перейти на ветку; ./clc.sh drop <ветка> [-f] — удалить локальную (без -f только слитую)
cmd_sw() { [[ -n "${1:-}" ]] || die "sw <ветка>"; git switch "$1"; }
cmd_drop() {
  local b=${1:-}
  [[ -n $b ]] || die "drop <ветка> [-f]"
  if [[ $b == "$(cur_branch)" || $b == main ]] || is_protected "$b"; then die "эту ветку не удаляю"; fi
  if [[ "${2:-}" == -f ]]; then git branch -D "$b"; else git branch -d "$b"; fi
}

# ------------------------------------------------ help
cmd_help() {
  cat <<EOF
Снимок и обмен с чатом
  ./${SELF} init                         завести .commits/, .claude, строки в .gitignore
  ./${SELF} look [--check]               обновить снимок в .claude
  ./${SELF} run '<команда>'              выполнить, вывод записать в .claude
  ./${SELF} push [be] [fe] [note] [--check]   пакет → .commits/pushed/, файл в буфер (ZOE_CLIP=uri|gnome|path)
  ./${SELF} apply [--force]              все patch*.zip из Downloads по очереди (снимок «до» в .commits/applied/)
  ./${SELF} rollback [list|N]            вернуть backend/ и frontend/ до применения N
Git
  ./${SELF} ship <be|fe|docs|all> "тип: что" [-b слово] [-p] [-f] [-n]
                                         ветка → apply → проверки → коммит → (-p) push пакета
  ./${SELF} commit <be|fe|docs|all> "тип: что" [-b слово] [-n]   проверки → коммит
  ./${SELF} branch <слово>               wip/<дата>-<слово> от свежего dev / последнего именованного коммита
  ./${SELF} next <слово>                 после PR: fetch + новая ветка от dev (не от старой wip!)
  ./${SELF} land "сводка" [-v R.B]       фича готова: wip-ветка → dev одним именованным коммитом (как PR + Squash and merge)
  ./${SELF} land --plan <файл> [-v R.B]  то же для готовой истории: по строке «<хэш> сводка» на фичу
  (ship ... -z "сводка" [-v R.B])        коммит и сразу land
  ./${SELF} amend [scope] ["сообщение"]  дополнить последний коммит (пока не на remote)
  ./${SELF} undo                         снять последний коммит, правки остаются в индексе
  ./${SELF} park [имя] | park list | unpark   спрятать / вернуть незакоммиченное
  ./${SELF} good [заметка] | good list   тесты+сборка → метка good/<время> на рабочей версии
  ./${SELF} restore [метка|коммит]       backend/ и frontend/ ← метка (по умолчанию последняя good)
  ./${SELF} publish                      git push -u origin <wip-ветка> (дальше PR в dev)
  ./${SELF} sw <ветка> | drop <ветка> [-f]     перейти / удалить локальную ветку
Папка загрузок: ZOE_DL=/путь
EOF
}

case "${1:-help}" in
  init)     cmd_init ;;
  look)     shift; cmd_look "$@" ;;
  run)      shift; cmd_run "$@" ;;
  push)     shift; cmd_push "$@" ;;
  apply)    shift; cmd_apply "$@" ;;
  rollback) shift; cmd_rollback "$@" ;;
  branch)   shift; cmd_branch "$@" ;;
  commit)   shift; cmd_commit "$@" ;;
  ship)     shift; cmd_ship "$@" ;;
  next)     shift; cmd_next "$@" ;;
  land)     shift; cmd_land "$@" ;;
  amend)    shift; cmd_amend "$@" ;;
  undo)     cmd_undo ;;
  park)     shift; cmd_park "$@" ;;
  unpark)   cmd_unpark ;;
  good)     shift; cmd_good "$@" ;;
  restore)  shift; cmd_restore "$@" ;;
  publish)  cmd_publish ;;
  sw)       shift; cmd_sw "$@" ;;
  drop)     shift; cmd_drop "$@" ;;
  help|-h|--help) cmd_help ;;
  *) cmd_help; exit 1 ;;
esac