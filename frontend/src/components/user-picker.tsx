import { useState, useRef, useEffect, useMemo } from "react";

interface SlackUser {
  id: string;
  name: string;
  email?: string;
  imageUrl?: string;
}

interface UserAvatarProps {
  user: SlackUser;
  size?: number;
}

function UserAvatar({ user, size = 20 }: UserAvatarProps) {
  const px = `${size}px`;
  if (user.imageUrl) {
    return (
      <img
        src={user.imageUrl}
        alt={user.name}
        className="rounded-full shrink-0"
        style={{ width: px, height: px }}
      />
    );
  }
  return (
    <span
      className="rounded-full bg-gray-300 flex items-center justify-center text-white font-medium shrink-0"
      style={{ width: px, height: px, fontSize: `${Math.round(size * 0.5)}px` }}
    >
      {user.name.charAt(0).toUpperCase()}
    </span>
  );
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
          className={`w-full min-h-[34px] flex flex-wrap items-center gap-1 px-1.5 py-1 border border-gray-300 rounded text-sm bg-white cursor-text ${disabled ? "opacity-50 cursor-not-allowed" : "hover:border-gray-400"}`}
        >
          {props.value.map((id) => {
            const u = userMap.get(id);
            if (!u) {
              return (
                <span
                  key={id}
                  className="inline-flex items-center gap-1 bg-gray-100 rounded px-1.5 py-0.5 text-xs"
                >
                  <span className="text-gray-500">{id}</span>
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      removeUser(id);
                    }}
                    className="text-gray-400 hover:text-gray-700"
                    aria-label="Remove"
                  >
                    ×
                  </button>
                </span>
              );
            }
            return (
              <span
                key={id}
                className="inline-flex items-center gap-1 bg-blue-50 border border-blue-100 rounded-full pl-0.5 pr-1.5 py-0.5 text-xs"
              >
                <UserAvatar user={u} size={18} />
                <span>{u.name}</span>
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    removeUser(id);
                  }}
                  className="text-blue-400 hover:text-blue-700"
                  aria-label="Remove"
                >
                  ×
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
            className="flex-1 min-w-[80px] px-1 py-0.5 text-sm border-none outline-none bg-transparent"
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

  // Single mode
  const selectedUser = props.value ? userMap.get(props.value) : undefined;

  return (
    <div className="relative" ref={containerRef}>
      {!isOpen ? (
        <button
          type="button"
          disabled={disabled}
          onClick={() => setIsOpen(true)}
          className="w-full flex items-center justify-between gap-2 px-2 py-1 border border-gray-300 rounded text-sm bg-white hover:border-gray-400 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {selectedUser ? (
            <span className="flex items-center gap-1.5 min-w-0">
              <UserAvatar user={selectedUser} size={20} />
              <span className="truncate">{selectedUser.name}</span>
            </span>
          ) : props.value ? (
            <span className="text-gray-500 truncate">{props.value}</span>
          ) : (
            <span className="text-gray-400">{placeholder}</span>
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
                className="text-gray-400 hover:text-gray-600 px-1"
                aria-label="Clear"
              >
                ×
              </span>
            )}
            <svg
              className="w-4 h-4 text-gray-400"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M19 9l-7 7-7-7"
              />
            </svg>
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
          className="w-full px-2 py-1 text-sm border border-blue-300 rounded focus:outline-none focus:ring-1 focus:ring-blue-500"
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
      className="absolute z-20 mt-1 w-full bg-white border border-gray-200 rounded-md shadow-lg max-h-60 overflow-y-auto py-1"
      role="listbox"
    >
      {users.length === 0 && (
        <li className="px-2 py-2 text-sm text-gray-400 text-center">
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
          className={`flex items-center gap-2 px-2 py-1.5 text-sm cursor-pointer ${
            i === highlightedIndex ? "bg-blue-50" : ""
          } ${selectedIds.includes(u.id) ? "font-medium" : ""}`}
          role="option"
          aria-selected={selectedIds.includes(u.id)}
        >
          <UserAvatar user={u} size={24} />
          <span className="truncate">{u.name}</span>
          {selectedIds.includes(u.id) && (
            <svg
              className="w-4 h-4 ml-auto text-blue-500 shrink-0"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M5 13l4 4L19 7"
              />
            </svg>
          )}
        </li>
      ))}
    </ul>
  );
};
