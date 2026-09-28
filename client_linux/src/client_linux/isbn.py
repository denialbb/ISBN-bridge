"""ISBN normalization and cryptographic/mathematical validation."""

class ISBNValidationError(ValueError):
    """Raised when an ISBN does not pass validation."""
    pass


def normalize(raw: str) -> str:
    """Normalize raw ISBN string: strips spaces, tabs, newlines, hyphens, and uppercases X."""
    res = []
    for c in raw:
        if c.isdigit():
            res.append(c)
        elif c in ('x', 'X'):
            res.append('X')
        elif c in (' ', '\t', '\r', '\n', '-'):
            continue
        else:
            res.append(c)
    return "".join(res)


def validate(raw: str) -> str:
    """Validate and normalize an ISBN-10 or ISBN-13 string.

    Returns the normalized digit string or raises ISBNValidationError.
    """
    s = normalize(raw)
    length = len(s)

    if length == 13:
        _validate_isbn13(s)
        return s
    elif length == 10:
        _validate_isbn10(s)
        return s
    else:
        raise ISBNValidationError(f"invalid ISBN length: {length}")


def _validate_isbn13(s: str) -> None:
    if not s.isdigit():
        raise ISBNValidationError("invalid characters in ISBN-13")

    total = 0
    for i, c in enumerate(s):
        digit = int(c)
        if i % 2 == 0:
            total += digit
        else:
            total += digit * 3

    if total % 10 != 0:
        raise ISBNValidationError("invalid ISBN-13 checksum")


def _validate_isbn10(s: str) -> None:
    for c in s[:9]:
        if not c.isdigit():
            raise ISBNValidationError("invalid characters in ISBN-10")

    last = s[9]
    if last == 'X':
        check_val = 10
    elif last.isdigit():
        check_val = int(last)
    else:
        raise ISBNValidationError("invalid check digit in ISBN-10")

    total = sum(int(s[i]) * (10 - i) for i in range(9)) + check_val
    if total % 11 != 0:
        raise ISBNValidationError("invalid ISBN-10 checksum")
