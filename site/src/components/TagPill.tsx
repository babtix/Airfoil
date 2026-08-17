import React from 'react';

interface TagPillProps {
  tag: string;
  isActive?: boolean;
  onClick?: (e: React.MouseEvent) => void;
  className?: string;
}

export const TagPill: React.FC<TagPillProps> = ({
  tag,
  isActive = false,
  onClick,
  className = ''
}) => {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`tagpill ${isActive ? 'tagpill-on' : ''} ${className}`}
      title={`Filter by ${tag}`}
    >
      {tag}
    </button>
  );
};
