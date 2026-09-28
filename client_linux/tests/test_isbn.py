import pytest
from client_linux.isbn import normalize, validate, ISBNValidationError

def test_normalize():
    assert normalize("978-0-306-40615-7") == "9780306406157"
    assert normalize(" 0 471 95869 7 ") == "0471958697"
    assert normalize("097522980x") == "097522980X"
    assert normalize("0-9752298-0-X") == "097522980X"

def test_validate_isbn13_valid():
    assert validate("978-0-306-40615-7") == "9780306406157"
    assert validate("9780471958697") == "9780471958697"

def test_validate_isbn10_valid():
    assert validate("0471958697") == "0471958697"
    assert validate("0-306-40615-2") == "0306406152"
    assert validate("097522980X") == "097522980X"
    assert validate("097522980x") == "097522980X"

def test_validate_invalid_checksum():
    with pytest.raises(ISBNValidationError, match="checksum"):
        validate("9780306406158")
    with pytest.raises(ISBNValidationError, match="checksum"):
        validate("0471958698")

def test_validate_invalid_length():
    with pytest.raises(ISBNValidationError, match="length"):
        validate("12345")
    with pytest.raises(ISBNValidationError, match="length"):
        validate("12345678901234")

def test_validate_invalid_characters():
    with pytest.raises(ISBNValidationError, match="character"):
        validate("978030640615A")
    with pytest.raises(ISBNValidationError, match="character|digit"):
        validate("047195869A")
    with pytest.raises(ISBNValidationError):
        validate("047195869;rm -rf")
