import { describe, it, expect } from 'vitest';
import { formatMinutes, validateDomainInput } from '../stores/appStore';

describe('FamilyControl Frontend Domain Helpers', () => {
  it('formats minutes accurately into compact desktop labels', () => {
    expect(formatMinutes(0)).toBe('0m');
    expect(formatMinutes(45)).toBe('45m');
    expect(formatMinutes(60)).toBe('1h');
    expect(formatMinutes(135)).toBe('2h 15m');
  });

  it('validates and normalizes domain names for allowlist/blocklist', () => {
    expect(validateDomainInput('https://www.TikTok.com/discover')).toBe('www.tiktok.com');
    expect(validateDomainInput('khanacademy.org')).toBe('khanacademy.org');
    expect(validateDomainInput('not a domain')).toBeNull();
    expect(validateDomainInput('http://-invalid.com')).toBeNull();
  });
});
