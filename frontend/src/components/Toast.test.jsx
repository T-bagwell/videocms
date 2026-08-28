import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import '@testing-library/jest-dom/vitest';
import Toast from './Toast.jsx';

describe('Toast', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it('renders the message', () => {
    render(<Toast message="Saved" onClose={() => {}} />);
    expect(screen.getByText('Saved')).toBeInTheDocument();
  });

  it('auto-closes after 2.6 seconds', () => {
    const onClose = vi.fn();
    render(<Toast message="Saved" onClose={onClose} />);
    expect(onClose).not.toHaveBeenCalled();
    vi.advanceTimersByTime(2600);
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
