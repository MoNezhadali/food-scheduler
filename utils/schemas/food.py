"module for defining food-related schemas using Pydantic"
# Built-in python modules
from typing import List, Optional

# Third-party modules
from pydantic import BaseModel


class Quantity(BaseModel):
    amount: float
    unit: str


class Ingredient(BaseModel):
    name: str
    quantity: Quantity


class Food(BaseModel):
    name: str
    display_name: str
    description: Optional[str] = None
    portions: int
    ingredients: List[Ingredient]
    recipe: List[str]
    labels: List[str]
