import { useState, useRef, useEffect, useMemo } from "react";
import { Avatar } from "./ui/avatar";
import { Icon } from "./ui/icon";

interface SlackUser {
  id: string;
  name: string;
  email?: string;
  imageUrl?: string;
}

interface BaseProps {
  users: SlackUser[];
  placeholder?: string;
  disabled?: boolean;
}

interface SingleProps extends BaseProps {
  multi?: false;
  value: string;
  onChange: (value: string) => void;
}

interface MultiProps extends BaseProps {
  multi: true;
  value: string[];
  onChange: (value: string[]) => void;
}

export function UserPicker(props: SingleProps | MultiProps) {
  const { users, placeholder = "Select a user...", disabled = false } = props;
  const [isOpen, setIsOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [highlightedIndex, setHighlightedIndex] = useState(0);
  const containerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLUListElement>(null);

  const userMap = useMemo(() => {
    const m = new Map<string, SlackUser>();
    for (const u of users) m.set(u.id, u);
    return m;
  }, [users]);

  const selectedIds: string[] = props.multi
    ? props.value
    : props.value
      ? [props.value]
      : [];

  const filteredUsers = useMemo(() => {
    const q = search.toLowerCase();
    const base = props.multi
      ? users.filter((u) => !selectedIds.includes(u.id))
      : users;
    if (!q) return base;
    return base.filter((u) => {
      return (
        u.name.toLowerCase().includes(q) ||
        u.id.toLowerCase().includes(q) ||
        (u.email?.toLowerCase().includes(q) ?? false)
      );
    });
  }, [users, search, selectedIds, props.multi]);

  useEffect(() => {
    setHighlightedIndex(0);
  }, [search, isOpen]);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(e.target as Node)
      ) {
        setIsOpen(false);
        setSearch("");
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  useEffect(() => {
    if (isOpen) inputRef.current?.focus();
  }, [isOpen]);

  useEffect(() => {
    if (!listRef.current) return;
    const item = listRef.current.children[highlightedIndex] as HTMLElement;
    if (item) item.scrollIntoView({ block: "nearest" });
  }, [highlightedIndex]);

  const selectUser = (user: SlackUser) => {
    if (props.multi) {
      props.onChange([...props.value, user.id]);
      setSearch("");
      inputRef.current?.focus();
    } else {
      props.onChange(user.id);
      setIsOpen(false);
      setSearch("");
    }
  };

  const removeUser = (userId: string) => {
    if (props.multi) {
      props.onChange(props.value.filter((id) => id !== userId));
    } else {
      props.onChange("");
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setHighlightedIndex((i) => Math.min(i + 1, filteredUsers.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setHighlightedIndex((i) => Math.max(i - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      const user = filteredUsers[highlightedIndex];
      if (user) selectUser(user);
    } else if (e.key === "Escape") {
      setIsOpen(false);
      setSearch("");
    } else if (
      e.key === "Backspace" &&
      props.multi &&
      !search &&
      props.value.length > 0
    ) {
      removeUser(props.value[props.value.length - 1]);
    }
  };

  if (props.multi) {
    return (
      <div className="relative" ref={containerRef}>
        <div
          onClick={() => {
            if (!disabled) {
              setIsOpen(true);
              inputRef.current?.focus();
            }
          }}
          className={`w-full min-h-[34px] flex flex-wrap items-center gap-1 px-1.5 py-1 border border-line-strong rounded-2 text-[13px] bg-bg-elev cursor-text ${disabled ? "opacity-50 cursor-not-allowed" : "hover:border-ink-4"}`}
        >
          {props.value.map((id) => {
            const u = userMap.get(id);
            const name = u?.name ?? id;
            return (
              <span
                key={id}
                className="inline-flex items-center gap-1 bg-bg-sunken border border-line rounded-2 pl-0.5 pr-1.5 py-0.5 text-[12px]"
              >
                {u && <Avatar name={name} src={u.imageUrl} size="xs" />}
                <span className="text-ink-1">{name}</span>
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    removeUser(id);
                  }}
                  className="text-ink-4 hover:text-ink-1"
                  aria-label="Remove"
                >
                  <Icon name="x" size={10} />
                </button>
              </span>
            );
          })}
          <input
            ref={inputRef}
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onFocus={() => setIsOpen(true)}
            onKeyDown={handleKeyDown}
            disabled={disabled}
            placeholder={props.value.length === 0 ? placeholder : ""}
            className="flex-1 min-w-[80px] px-1 py-0.5 text-[13px] border-none outline-none bg-transparent text-ink-1"
          />
        </div>

        {isOpen && (
          <UserList
            ref={listRef}
            users={filteredUsers}
            selectedIds={selectedIds}
            highlightedIndex={highlightedIndex}
            onHighlight={setHighlightedIndex}
            onSelect={selectUser}
          />
        )}
      </div>
    );
  }

  // Single
  const selectedUser = props.value ? userMap.get(props.value) : undefined;

  return (
    <div className="relative" ref={containerRef}>
      {!isOpen ? (
        <button
          type="button"
          disabled={disabled}
          onClick={() => setIsOpen(true)}
          className="w-full flex items-center justify-between gap-2 px-2 py-1 border border-line-strong rounded-2 text-[13px] bg-bg-elev hover:border-ink-4 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {selectedUser ? (
            <span className="flex items-center gap-1.5 min-w-0">
              <Avatar name={selectedUser.name} src={selectedUser.imageUrl} size="sm" />
              <span className="truncate text-ink-1">{selectedUser.name}</span>
            </span>
          ) : props.value ? (
            <span className="text-ink-3 truncate">{props.value}</span>
          ) : (
            <span className="text-ink-4">{placeholder}</span>
          )}
          <span className="flex items-center gap-1 shrink-0">
            {props.value && (
              <span
                role="button"
                tabIndex={0}
                onClick={(e) => {
                  e.stopPropagation();
                  removeUser(props.value);
                }}
                className="text-ink-4 hover:text-ink-1 px-0.5"
                aria-label="Clear"
              >
                <Icon name="x" size={11} />
              </span>
            )}
            <Icon name="chevron" size={12} className="text-ink-4" />
          </span>
        </button>
      ) : (
        <input
          ref={inputRef}
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Type to search..."
          className="w-full px-2 py-1 text-[13px] border border-brand rounded-2 bg-bg-elev text-ink-1 focus:outline-none focus:ring-2 focus:ring-brand-soft"
        />
      )}

      {isOpen && (
        <UserList
          ref={listRef}
          users={filteredUsers}
          selectedIds={selectedIds}
          highlightedIndex={highlightedIndex}
          onHighlight={setHighlightedIndex}
          onSelect={selectUser}
        />
      )}
    </div>
  );
}

interface UserListProps {
  users: SlackUser[];
  selectedIds: string[];
  highlightedIndex: number;
  onHighlight: (i: number) => void;
  onSelect: (user: SlackUser) => void;
}

const UserList = ({
  ref,
  users,
  selectedIds,
  highlightedIndex,
  onHighlight,
  onSelect,
}: UserListProps & { ref: React.RefObject<HTMLUListElement | null> }) => {
  return (
    <ul
      ref={ref}
      className="absolute z-30 mt-1 w-full bg-bg-elev border border-line rounded-3 shadow-pop max-h-60 overflow-y-auto p-1"
      role="listbox"
    >
      {users.length === 0 && (
        <li className="px-2 py-2 text-[13px] text-ink-4 text-center">
          No users found
        </li>
      )}
      {users.map((u, i) => (
        <li
          key={u.id}
          onMouseDown={(e) => {
            e.preventDefault();
            onSelect(u);
          }}
          onMouseEnter={() => onHighlight(i)}
          className={`flex items-center gap-2 px-2 py-1.5 rounded-2 text-[13px] cursor-pointer ${
            i === highlightedIndex ? "bg-bg-sunken" : ""
          } ${selectedIds.includes(u.id) ? "font-medium" : ""}`}
          role="option"
          aria-selected={selectedIds.includes(u.id)}
        >
          <Avatar name={u.name} src={u.imageUrl} size="md" />
          <span className="truncate text-ink-1">{u.name}</span>
          {selectedIds.includes(u.id) && (
            <Icon name="check" size={13} className="ml-auto text-brand shrink-0" />
          )}
        </li>
      ))}
    </ul>
  );
};
